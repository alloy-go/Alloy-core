package services

import (
    "context"
    "log"
    "strings"
    
    "time"
    appsv1 "k8s.io/api/apps/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
    "github.com/jackc/pgx/v5/pgxpool"
)

type DeploymentWatcher struct {
    DB *pgxpool.Pool
}

func NewDeploymentWatcher(db *pgxpool.Pool) *DeploymentWatcher {
    return &DeploymentWatcher{DB: db}
}

func (dw *DeploymentWatcher) Start() {
    log.Println("🔍 Kubernetes Deployment Watcher started - INSTANT failure detection!")
    
    go func() {
        // Watch ALL namespaces (or specify yours)
        if err := dw.watchAllDeployments(); err != nil {
            log.Printf("❌ Watcher error: %v", err)
        }
    }()
}

func (dw *DeploymentWatcher) watchAllDeployments() error {
    // Load kubeconfig from DB users table (your existing logic)
    var kubeconfigPath, defaultContext string
    err := dw.DB.QueryRow(context.Background(), `
        SELECT u.config_path, p.context_name
        FROM users u
        JOIN projects p ON u.user_id = p.user_id
        LIMIT 1
    `).Scan(&kubeconfigPath, &defaultContext)

    if err != nil {
        return err
    }

    config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
        &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
        &clientcmd.ConfigOverrides{CurrentContext: defaultContext},
    ).ClientConfig()
    if err != nil {
        return err
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return err
    }

    // Watch ALL deployments across ALL namespaces
    for {
        watcher, err := clientset.AppsV1().Deployments(metav1.NamespaceAll).Watch(context.Background(), metav1.ListOptions{})
        if err != nil {
            log.Printf("❌ Watch failed, retrying in 5s: %v", err)
            time.Sleep(5 * time.Second)
            continue
        }

        log.Println("👁️  Watching all deployments... (Ctrl+C to stop)")
        
        for event := range watcher.ResultChan() {
            deployment := event.Object.(*appsv1.Deployment)
            
            // Skip non-failed deployments
            if !dw.isDeploymentFailed(deployment) {
                continue
            }

            log.Printf("🚨 DETECTED FAILED DEPLOYMENT: %s/%s", deployment.Namespace, deployment.Name)
            
            // INSTANT rollback trigger
            if err := dw.triggerRollbackForDeployment(deployment); err != nil {
                log.Printf("❌ Rollback failed for %s: %v", deployment.Name, err)
            }
        }
        
        log.Println("🔄 Watch connection lost, reconnecting...")
    }
}

func (dw *DeploymentWatcher) isDeploymentFailed(deployment *appsv1.Deployment) bool {
    // Check deployment conditions for failure
    for _, condition := range deployment.Status.Conditions {
        if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == "True" {
            return true
        }
        if condition.Type == appsv1.DeploymentProgressing && condition.Status == "False" {
            if strings.Contains(condition.Message, "FailedCreate") || 
               strings.Contains(condition.Message, "timeout") {
                return true
            }
        }
    }
    
    return false
}

func (dw *DeploymentWatcher) triggerRollbackForDeployment(deployment *appsv1.Deployment) error {
    // Find matching deployment record in YOUR DB
    var deploymentID, projectID, userID string
    err := dw.DB.QueryRow(context.Background(), `
        SELECT d.deployment_id, p.project_id, p.user_id
        FROM deployments d
        JOIN projects p ON d.project_id = p.project_id
        WHERE d.deployment_name = $1 AND d.namespace = $2 
          AND d.status = 'failed' AND d.deployment_type = 'deploy'
        ORDER BY d.created_at DESC LIMIT 1
    `, deployment.Name, deployment.Namespace).Scan(&deploymentID, &projectID, &userID)
    
    if err != nil {
        log.Printf("⚠️  No DB record for failed deployment %s/%s", deployment.Namespace, deployment.Name)
        return nil
    }

    // Mark as hard failure if not already
    _, _ = dw.DB.Exec(context.Background(), `
        UPDATE deployments 
        SET failure_type = 'hard', needs_rollback = true
        WHERE deployment_id = $1
    `, deploymentID)

    // INSTANT ROLLBACK (no 30s delay!)
    req := RollbackRequest{
        ProjectID:          projectID,
        UserID:             userID,
        FailedDeploymentID: deploymentID,
    }

    result, err := ExecuteRollback(dw.DB, req)
    if err != nil {
        log.Printf("❌ Instant rollback FAILED %s: %v", deploymentID, err)
        return err
    }

    log.Printf("✅ INSTANT ROLLBACK: %s → %s (%s/%s)", 
        deploymentID, result.RollbackDeploymentID, deployment.Namespace, deployment.Name)
    
    return nil
}
