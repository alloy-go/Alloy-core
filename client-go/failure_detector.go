package services

import (
    "context"
    "fmt"
    "strings"
    
    "github.com/jackc/pgx/v5/pgxpool"
    
    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
)

type FailureClassification struct {
    Type          string
    Reason        string
    NeedsRollback bool
}

// ClassifyDeploymentFailure - Call from monitorDeploymentStatus
func ClassifyDeploymentFailure(kubeconfigPath, contextName, namespace, deploymentName string) (*FailureClassification, error) {
    config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
        &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
        &clientcmd.ConfigOverrides{CurrentContext: contextName},
    ).ClientConfig()
    if err != nil {
        return nil, err
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, err
    }

    // Quick pod check for crashloop/imagepull (HARD failures)
    pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
        LabelSelector: fmt.Sprintf("app=%s", deploymentName),
    })
    if err != nil {
        return &FailureClassification{"soft", "pod-list-error", true}, nil
    }

    crashLoop := 0
    imagePull := 0
    for _, pod := range pods.Items {
        if pod.Status.Phase == v1.PodFailed || pod.Status.Phase == v1.PodPending {
            for _, status := range pod.Status.ContainerStatuses {
                if status.State.Waiting != nil {
                    if strings.Contains(status.State.Waiting.Reason, "CrashLoopBackOff") {
                        crashLoop++
                    }
                    if strings.Contains(status.State.Waiting.Reason, "ImagePull") {
                        imagePull++
                    }
                }
            }
        }
    }

    if crashLoop > 0 {
        return &FailureClassification{"hard", fmt.Sprintf("crashloop:%d", crashLoop), true}, nil
    }
    if imagePull > 0 {
        return &FailureClassification{"hard", fmt.Sprintf("imagepull:%d", imagePull), true}, nil
    }

    return &FailureClassification{"soft", "deployment-failed", true}, nil
}

// UpdateDeploymentFailure - Store classification in DB
func UpdateDeploymentFailure(db *pgxpool.Pool, deploymentID string, classification *FailureClassification) error {
    _, err := db.Exec(context.Background(), `
        UPDATE deployments 
        SET status = 'failed',
            failure_type = $1,
            needs_rollback = $2,
            error_message = $3,
            updated_at = NOW()
        WHERE deployment_id = $4
    `, classification.Type, classification.NeedsRollback, classification.Reason, deploymentID)
    return err
}
