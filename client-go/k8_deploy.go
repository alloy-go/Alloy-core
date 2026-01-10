package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

type DeploymentResult struct {
    Secret     string `json:"secret"`
    Service    string `json:"service"`
    Deployment string `json:"deployment"`
}

type K8sDeployConfig struct {
    KubeconfigPath string
    ContextName    string
    SecretYAML     []byte
    ServiceYAML    []byte
    DeploymentYAML []byte
	DeploymentID    string
	Namespace 		string
	DeploymentName	string
	DB				*pgxpool.Pool
}

// DeployToKubernetes handles the full deployment process
func DeployToKubernetes(config K8sDeployConfig) (*DeploymentResult, error) {
    results := &DeploymentResult{}

    // Update status to deploying
    updateDeploymentStatus(config.DB, config.DeploymentID, "deploying", "", "", "")

    // Deploy Secret
    log.Println("📦 Deploying Secret...")
    if err := ApplyYAML(config.KubeconfigPath, config.ContextName, config.SecretYAML); err != nil {
        log.Printf("❌ Secret deployment failed: %v\n", err)
        results.Secret = "failed"
        updateDeploymentStatus(config.DB, config.DeploymentID, "deploying", "failed", "", "")
    } else {
        log.Println("✅ Secret deployed")
        results.Secret = "success"
        updateDeploymentStatus(config.DB, config.DeploymentID, "deploying", "success", "", "")
    }

    // Deploy Service
    log.Println("📦 Deploying Service...")
    if err := ApplyYAML(config.KubeconfigPath, config.ContextName, config.ServiceYAML); err != nil {
        log.Printf("❌ Service deployment failed: %v\n", err)
        results.Service = "failed"
        updateDeploymentStatus(config.DB, config.DeploymentID, "deploying", results.Secret, "failed", "")
    } else {
        log.Println("✅ Service deployed")
        results.Service = "success"
        updateDeploymentStatus(config.DB, config.DeploymentID, "deploying", results.Secret, "success", "")
    }

    // Deploy Deployment
    log.Println("📦 Deploying Deployment...")
    if err := ApplyYAML(config.KubeconfigPath, config.ContextName, config.DeploymentYAML); err != nil {
        log.Printf("❌ Deployment failed: %v\n", err)
        results.Deployment = "failed"
        updateDeploymentStatus(config.DB, config.DeploymentID, "failed", results.Secret, results.Service, "failed")
        updateDeploymentError(config.DB, config.DeploymentID, err.Error())
        return results, fmt.Errorf("deployment failed: %w", err)
    }
    
    log.Println("✅ Deployment created")
    results.Deployment = "success"
    updateDeploymentStatus(config.DB, config.DeploymentID, "deploying", results.Secret, results.Service, "success")

    // Start monitoring deployment status in background
    go monitorDeploymentStatus(config)

    return results, nil
}

// Monitor deployment status until pods are ready
func monitorDeploymentStatus(config K8sDeployConfig) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    timeout := time.After(5 * time.Minute)

    for {
        select {
        case <-timeout:
            log.Printf("⏱️  Deployment monitoring timeout for %s\n", config.DeploymentName)
            updateDeploymentStatus(config.DB, config.DeploymentID, "failed", "", "", "")
            updateDeploymentError(config.DB, config.DeploymentID, "deployment timeout")
            return

        case <-ticker.C:
            status, err := GetDeploymentStatus(
                config.KubeconfigPath,
                config.ContextName,
                config.Namespace,
                config.DeploymentName,
            )

            if err != nil {
                log.Printf("❌ Failed to get deployment status: %v\n", err)
                continue
            }

            log.Printf("📊 Deployment %s status: %s\n", config.DeploymentName, status)

            if status == "ready" {
                log.Printf("✅ Deployment %s is ready!\n", config.DeploymentName)
                updateDeploymentStatus(config.DB, config.DeploymentID, "ready", "", "", "")
                return
            }

            if status == "failed" {
                log.Printf("❌ Deployment %s failed\n", config.DeploymentName)
                updateDeploymentStatus(config.DB, config.DeploymentID, "failed", "", "", "")
                return
            }
        }
    }
}

// Update deployment status in database
func updateDeploymentStatus(db *pgxpool.Pool, deploymentID, status, secretStatus, serviceStatus, deploymentStatus string) {
    ctx := context.Background()
    
    query := `UPDATE deployments SET status = $1, updated_at = NOW()`
    args := []interface{}{status}
    argCount := 2

    if secretStatus != "" {
        query += fmt.Sprintf(`, secret_status = $%d`, argCount)
        args = append(args, secretStatus)
        argCount++
    }
    if serviceStatus != "" {
        query += fmt.Sprintf(`, service_status = $%d`, argCount)
        args = append(args, serviceStatus)
        argCount++
    }
    if deploymentStatus != "" {
        query += fmt.Sprintf(`, deployment_status = $%d`, argCount)
        args = append(args, deploymentStatus)
        argCount++
    }

    query += fmt.Sprintf(` WHERE deployment_id = $%d`, argCount)
    args = append(args, deploymentID)

    _, err := db.Exec(ctx, query, args...) // Add ctx here
    if err != nil {
        log.Printf("❌ Failed to update deployment status: %v\n", err)
    }
}

func updateDeploymentError(db *pgxpool.Pool, deploymentID, errorMsg string) {
    ctx := context.Background()
    
    // Truncate error message if too long and escape special chars
    if len(errorMsg) > 500 {
        errorMsg = errorMsg[:500] + "..."
    }
    
    _, err := db.Exec(ctx, `
        UPDATE deployments 
        SET error_message = $1, updated_at = NOW()
        WHERE deployment_id = $2
    `, errorMsg, deploymentID)
    
    if err != nil {
        log.Printf("❌ Failed to update error message: %v\n", err)
    }
}


// ApplyYAML applies any Kubernetes YAML to the cluster
func ApplyYAML(kubeconfigPath, contextName string, yamlContent []byte) error {
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

// GetDeploymentStatus checks the status of a deployment
func GetDeploymentStatus(kubeconfigPath, contextName, namespace, deploymentName string) (string, error) {
    config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
        &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
        &clientcmd.ConfigOverrides{CurrentContext: contextName},
    ).ClientConfig()
    if err != nil {
        return "", err
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return "", err
    }

    deployment, err := clientset.AppsV1().Deployments(namespace).Get(
        context.Background(),
        deploymentName,
        metav1.GetOptions{},
    )
    if err != nil {
        return "", err
    }

    // Check deployment conditions
    for _, condition := range deployment.Status.Conditions {
        if condition.Type == "Available" && condition.Status == "True" {
            if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
                return "ready", nil
            }
        }
        if condition.Type == "Progressing" && condition.Reason == "ProgressDeadlineExceeded" {
            return "failed", nil
        }
    }

    return "progressing", nil
}
