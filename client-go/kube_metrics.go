package services

import (
	"context"
	"fmt"

	"github.com/Santhoshkumar044/MiniMon/models"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeMetricsService handles Kubernetes metrics collection via client-go
type KubeMetricsService struct {
	// No global state - creates clientset per request using kubeconfig from DB
}

// NewKubeMetricsService creates a new Kubernetes metrics service
func NewKubeMetricsService() *KubeMetricsService {
	return &KubeMetricsService{}
}

// GetKubernetesMetrics fetches pod and deployment metrics from Kubernetes API
func (s *KubeMetricsService) GetKubernetesMetrics(ctx context.Context, req models.MetricsRequest) (*models.KubernetesMetrics, error) {
	// Create Kubernetes clientset from kubeconfig path
	clientset, err := s.createClientset(req.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s clientset: %w", err)
	}

	// Fetch deployment status
	deploymentStatus, err := s.getDeploymentStatus(ctx, clientset, req.Namespace, req.DeploymentName)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment status: %w", err)
	}

	// Fetch pod metrics
	podMetrics, err := s.getPodMetrics(ctx, clientset, req.Namespace, req.DeploymentName)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod metrics: %w", err)
	}

	// Optional: Get resource metrics if metrics-server is available
	// resourceMetrics, _ := s.getResourceMetrics(ctx, clientset, req.Namespace, req.DeploymentName)

	return &models.KubernetesMetrics{
		Pods:       *podMetrics,
		Deployment: *deploymentStatus,
		Resources:  nil, // Set if metrics-server available
	}, nil
}

// createClientset creates a Kubernetes clientset from kubeconfig
func (s *KubeMetricsService) createClientset(cfg models.KubeConfig) (*kubernetes.Clientset, error) {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.KubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: cfg.ContextName},
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kubeconfig load error: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("clientset creation error: %w", err)
	}

	return clientset, nil
}

// getDeploymentStatus fetches deployment replica counts and status
func (s *KubeMetricsService) getDeploymentStatus(ctx context.Context, clientset *kubernetes.Clientset, namespace, deploymentName string) (*models.DeploymentStatus, error) {
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	status := "healthy"
	if deployment.Status.UnavailableReplicas > 0 {
		status = "degraded"
	}
	if deployment.Status.AvailableReplicas == 0 {
		status = "critical"
	}

	// Check for progressing condition
	for _, cond := range deployment.Status.Conditions {
		if cond.Type == "Progressing" && cond.Status == v1.ConditionFalse {
			status = "failed"
			break
		}
	}

	var replicas int32
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	return &models.DeploymentStatus{
		ReplicasDesired:   replicas,
		ReplicasAvailable: deployment.Status.AvailableReplicas,
		ReplicasReady:     deployment.Status.ReadyReplicas,
		Status:            status,
	}, nil
}

// getPodMetrics fetches pod counts, restarts, and details
func (s *KubeMetricsService) getPodMetrics(ctx context.Context, clientset *kubernetes.Clientset, namespace, deploymentName string) (*models.PodMetrics, error) {
	// List pods for this deployment
	labelSelector := fmt.Sprintf("app=%s", deploymentName)
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var totalRestarts int32
	var readyCount int32
	podDetails := make([]models.PodDetails, 0, len(pods.Items))

	for _, pod := range pods.Items {
		// Count container restarts
		var podRestarts int32
		for _, containerStatus := range pod.Status.ContainerStatuses {
			podRestarts += containerStatus.RestartCount
		}
		totalRestarts += podRestarts

		// Check if pod is ready
		ready := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
				ready = true
				readyCount++
				break
			}
		}

		podDetails = append(podDetails, models.PodDetails{
			Name:      pod.Name,
			Status:    string(pod.Status.Phase),
			Ready:     ready,
			Restarts:  podRestarts,
			CreatedAt: pod.CreationTimestamp.Time,
			NodeName:  pod.Spec.NodeName,
		})
	}

	return &models.PodMetrics{
		Total:    int32(len(pods.Items)),
		Ready:    readyCount,
		Restarts: totalRestarts,
		Details:  podDetails,
	}, nil
}
