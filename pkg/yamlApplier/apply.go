package yaml

import (
	"context"
	"fmt"
	"strings"
	"log"
	
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/discovery/cached/memory"
)

func Apply(kubeconfigPath, contextName string, yamlContent []byte) error {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
	if err != nil {
		return fmt.Errorf("kubeconfig load error: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("dynamic client error: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("clientset error: %w", err)
	}

	decUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, gvk, err := decUnstructured.Decode(yamlContent, nil, obj)
	if err != nil {
		return fmt.Errorf("yaml decode error: %w", err)
	}

	discoveryClient := memory.NewMemCacheClient(clientset.Discovery())
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("rest mapping error: %w", err)
	}

	namespace := obj.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	ctx := context.Background()
	var dr dynamic.ResourceInterface

	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		dr = dynamicClient.Resource(mapping.Resource).Namespace(namespace)
	} else {
		dr = dynamicClient.Resource(mapping.Resource)
	}

	_, err = dr.Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			_, err = dr.Update(ctx, obj, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("update error: %w", err)
			}
			log.Printf("   ↻ Updated existing %s/%s\n", gvk.Kind, obj.GetName())
			return nil
		}
		return fmt.Errorf("create error: %w", err)
	}

	log.Printf("   ✓ Created %s/%s\n", gvk.Kind, obj.GetName())
	return nil
}

