package services

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type PortForwardRequest struct {
	RestConfig  *rest.Config
	Namespace   string
	ServiceName string
	LocalPort   int
	RemotePort  int
	StopCh      <-chan struct{}
	ReadyCh     chan struct{}
}

var (
	activePortForwards = make(map[string]chan struct{})
	portForwardMutex   sync.RWMutex
)

// StopPortForward gracefully stops an existing port-forward
func StopPortForward(key string) {
	portForwardMutex.Lock()
	defer portForwardMutex.Unlock()

	if stopCh, exists := activePortForwards[key]; exists {
		log.Printf("🛑 Stopping existing port-forward: %s\n", key)
		close(stopCh) // This stops the port-forward goroutine
		delete(activePortForwards, key)

		// Give it time to clean up
		time.Sleep(2 * time.Second)
		log.Printf("✅ Port-forward stopped: %s\n", key)
	}
}

// RegisterPortForward saves a port-forward stop channel
func RegisterPortForward(key string, stopCh chan struct{}) {
	portForwardMutex.Lock()
	defer portForwardMutex.Unlock()
	activePortForwards[key] = stopCh
}

// PortForwardToService creates a port-forward to a Kubernetes service
// PortForwardToService creates a port-forward to a Kubernetes service
func PortForwardToService(req PortForwardRequest) error {
	// Get clientset
	clientset, err := kubernetes.NewForConfig(req.RestConfig)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// Get service to find a pod
	service, err := clientset.CoreV1().Services(req.Namespace).Get(
		context.Background(),
		req.ServiceName,
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to get service: %w", err)
	}

	// Find pods matching service selector
	podList, err := clientset.CoreV1().Pods(req.Namespace).List(
		context.Background(),
		metav1.ListOptions{
			LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
				MatchLabels: service.Spec.Selector,
			}),
		},
	)
	if err != nil || len(podList.Items) == 0 {
		return fmt.Errorf("no pods found for service: %w", err)
	}

	// Use the NEWEST running pod (highest creation timestamp)
	var targetPod *v1.Pod
	var newestTime metav1.Time

	for i := range podList.Items {
		pod := &podList.Items[i]

		// Only consider running and ready pods
		if pod.Status.Phase != v1.PodRunning {
			continue
		}

		// Check if pod is actually ready
		isPodReady := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				isPodReady = true
				break
			}
		}

		if !isPodReady {
			continue
		}

		// Check for deletion timestamp (pod is being terminated)
		if pod.DeletionTimestamp != nil {
			log.Printf("⚠️  Skipping pod %s (being deleted)\n", pod.Name)
			continue
		}

		// Select the newest pod
		if targetPod == nil || pod.CreationTimestamp.After(newestTime.Time) {
			targetPod = pod
			newestTime = pod.CreationTimestamp
		}
	}

	if targetPod == nil {
		return fmt.Errorf("no ready pods found (all may be terminating)")
	}

	log.Printf("🎯 Selected pod: %s (created: %s)\n", targetPod.Name, targetPod.CreationTimestamp)

	// Setup port-forward
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		targetPod.Namespace, targetPod.Name)

	hostIP := strings.TrimLeft(req.RestConfig.Host, "htps:/")

	transport, upgrader, err := spdy.RoundTripperFor(req.RestConfig)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(
		upgrader,
		&http.Client{Transport: transport},
		http.MethodPost,
		&url.URL{Scheme: "https", Path: path, Host: hostIP},
	)

	var berr, bout bytes.Buffer
	buffErr := bufio.NewWriter(&berr)
	buffOut := bufio.NewWriter(&bout)

	fw, err := portforward.New(
		dialer,
		[]string{fmt.Sprintf("%d:%d", req.LocalPort, req.RemotePort)},
		req.StopCh,
		req.ReadyCh,
		buffOut,
		buffErr,
	)
	if err != nil {
		return err
	}

	return fw.ForwardPorts()
}

// StartPortForward initiates port-forwarding with cleanup
func StartPortForward(kubeconfigPath, contextName, namespace, serviceName string, localPort, remotePort int) error {
	// Create unique key for this port-forward
	pfKey := fmt.Sprintf("%s-%s-%d", namespace, serviceName, localPort)

	// Stop any existing port-forward on this port
	log.Printf("🔍 Checking for existing port-forward on %d...\n", localPort)
	StopPortForward(pfKey)

	// Load kubeconfig
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
	if err != nil {
		return fmt.Errorf("kubeconfig error: %w", err)
	}

	stopCh := make(chan struct{}, 1)
	readyCh := make(chan struct{})

	// Register this port-forward BEFORE starting it
	RegisterPortForward(pfKey, stopCh)

	// Start port-forward in goroutine
	go func() {
		defer func() {
			// Cleanup on exit
			portForwardMutex.Lock()
			delete(activePortForwards, pfKey)
			portForwardMutex.Unlock()
			log.Printf("🔌 Port-forward goroutine exited for %s\n", serviceName)
		}()

		req := PortForwardRequest{
			RestConfig:  config,
			Namespace:   namespace,
			ServiceName: serviceName,
			LocalPort:   localPort,
			RemotePort:  remotePort,
			StopCh:      stopCh,
			ReadyCh:     readyCh,
		}

		if err := PortForwardToService(req); err != nil {
			log.Printf("❌ Port-forward failed: %v\n", err)
		}
	}()

	// Wait for ready
	select {
	case <-readyCh:
		log.Printf("✅ Port-forwarding active: localhost:%d -> %s:%d\n",
			localPort, serviceName, remotePort)
		return nil
	case <-time.After(10 * time.Second):
		close(stopCh)
		return fmt.Errorf("port-forward timeout")
	}
}
