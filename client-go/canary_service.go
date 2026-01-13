package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
)

type CanaryPipelineConfig struct {
	KubeconfigPath     string
	ContextName        string
	Namespace          string
	DeploymentName     string
	CanaryDeploymentID string
	CanaryDeployment   string
	ImageTag           string
	SecretYAML         []byte
	ServiceYAML        []byte
	DeploymentYAML     []byte
	StableReplicas     int
}

type CanaryStage struct {
	Replicas    int
	TrafficPct  int
	DurationMin int
}

var canaryStages = []CanaryStage{
	{Replicas: 1, TrafficPct: 17, DurationMin: 3},
	{Replicas: 3, TrafficPct: 50, DurationMin: 3},
	{Replicas: 5, TrafficPct: 83, DurationMin: 2},
}

// 🚀 MAIN ORCHESTRATOR
func StartCanaryPipeline(db *pgxpool.Pool, config CanaryPipelineConfig) {
	log.Printf("🔄 Starting canary pipeline for %s", config.CanaryDeploymentID)

	// Update DB: pipeline started
	_, _ = db.Exec(context.Background(),
		`UPDATE deployments SET status='starting', updated_at=NOW() WHERE deployment_id=$1`,
		config.CanaryDeploymentID)

	// Apply secret and service first (these are shared between canary and stable)
	if len(config.SecretYAML) > 0 {
		if err := ApplyYAML(config.KubeconfigPath, config.ContextName, config.SecretYAML); err != nil {
			log.Printf("❌ Failed to apply secret: %v", err)
		}
	}
	if len(config.ServiceYAML) > 0 {
		// Modify service to route to BOTH canary and stable pods
		modifiedServiceYAML := modifyServiceForCanary(config.ServiceYAML)
		if err := ApplyYAML(config.KubeconfigPath, config.ContextName, modifiedServiceYAML); err != nil {
			log.Printf("❌ Failed to apply service: %v", err)
		}
	}

	for stageIdx, stage := range canaryStages {
		log.Printf("🟡 Stage %d/%d: Scaling to %d replicas (%.0f%% traffic)",
			stageIdx+1, len(canaryStages), stage.Replicas, float64(stage.TrafficPct))

		if !canaryStagePass(db, config, stageIdx, stage) {
			log.Printf("🛑 CANARY FAILED at stage %d", stageIdx+1)
			abortCanary(db, config)
			return
		}
		log.Printf("✅ Stage %d PASSED", stageIdx+1)
	}

	promoteCanaryToStable(db, config)
}

func canaryStagePass(db *pgxpool.Pool, config CanaryPipelineConfig, stageIdx int, stage CanaryStage) bool {
	// 1. Update DB stage
	_, _ = db.Exec(context.Background(), `
        UPDATE deployments SET canary_stage=$1, status='analyzing', updated_at=NOW()
        WHERE deployment_id=$2`, stageIdx, config.CanaryDeploymentID)

	// 2. Generate & apply canary deployment YAML
	canaryYAML := modifyDeploymentForCanary(config.DeploymentYAML,
		config.CanaryDeployment, config.ImageTag, stage.Replicas, "canary")

	if err := ApplyYAML(config.KubeconfigPath, config.ContextName, canaryYAML); err != nil {
		log.Printf("❌ Failed to deploy canary stage %d: %v", stageIdx, err)
		UpdateCanaryErrmsg(db, config.CanaryDeploymentID, fmt.Sprintf("stage-%d-deploy", stageIdx))
		return false
	}

	log.Printf("📦 Canary stage %d deployed (%d replicas)", stageIdx, stage.Replicas)

	// 3. Monitor using existing function
	deadline := time.Now().Add(time.Duration(stage.DurationMin*60) * time.Second)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		status, err := GetDeploymentStatus(config.KubeconfigPath, config.ContextName,
			config.Namespace, config.CanaryDeployment)
		if err != nil {
			log.Printf("⚠️ Status check failed: %v", err)
			<-ticker.C
			continue
		}

		log.Printf("📊 Canary: %s (%.1f min left)", status, time.Until(deadline).Minutes())

		if status == "ready" {
			_, _ = db.Exec(context.Background(),
				`UPDATE deployments SET status='stage_passed' WHERE deployment_id=$1`,
				config.CanaryDeploymentID)
			return true
		}
		if status == "failed" {
			UpdateCanaryErrmsg(db, config.CanaryDeploymentID, fmt.Sprintf("stage-%d-failed", stageIdx))
			return false
		}
		<-ticker.C
	}

	log.Printf("⏱️ Stage %d timed out", stageIdx)
	UpdateCanaryErrmsg(db, config.CanaryDeploymentID, fmt.Sprintf("stage-%d-timeout", stageIdx))
	return false
}

func abortCanary(db *pgxpool.Pool, config CanaryPipelineConfig) {
	log.Println("🛑 ABORTING CANARY DEPLOYMENT...")

	_, _ = db.Exec(context.Background(), `
        UPDATE deployments SET status='canary_aborted', failure_type='soft', updated_at=NOW()
        WHERE deployment_id=$1`, config.CanaryDeploymentID)

	// Scale canary to 0 for cleanup
	cleanupYAML := modifyDeploymentForCanary(config.DeploymentYAML,
		config.CanaryDeployment, config.ImageTag, 0, "canary")
	ApplyYAML(config.KubeconfigPath, config.ContextName, cleanupYAML)

	log.Println("🗑️ Canary ABORTED and cleaned up ✅")
}
func PromoteStableByUpdatingExisting(kubeconfigPath, contextName, namespace, stableName, newImage string, replicas int) error {
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
	if err != nil {
		return fmt.Errorf("kubeconfig load error: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("clientset error: %w", err)
	}

	dep, err := clientset.AppsV1().Deployments(namespace).Get(context.Background(), stableName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get stable deployment error: %w", err)
	}

	// Only mutate allowed fields
	dep.Spec.Replicas = ptr.To(int32(replicas))

	if len(dep.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("stable deployment has 0 containers")
	}
	// No hardcode: update first container (works for single-container pods)
	dep.Spec.Template.Spec.Containers[0].Image = newImage

	_, err = clientset.AppsV1().Deployments(namespace).Update(context.Background(), dep, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update stable deployment error: %w", err)
	}
	return nil
}

func promoteCanaryToStable(db *pgxpool.Pool, config CanaryPipelineConfig) {
	log.Println("🚀 PROMOTING TO PRODUCTION...")

	// Update stable deployment with new image (WITHOUT changing selector)
	// This avoids "selector is immutable" error
	// stableYAML := updateStableDeploymentImage(config.DeploymentYAML,
	//     config.ImageTag, config.StableReplicas)

	// if err := ApplyYAML(config.KubeconfigPath, config.ContextName, stableYAML); err != nil {
	//     log.Printf("❌ Failed to promote to stable: %v", err)
	//     UpdateCanaryErrmsg(db, config.CanaryDeploymentID, "promotion-failed")
	//     return
	// }
	if err := PromoteStableByUpdatingExisting(
		config.KubeconfigPath,
		config.ContextName,
		config.Namespace,
		config.DeploymentName, // "student-app"
		config.ImageTag,
		config.StableReplicas,
	); err != nil {
		log.Printf("❌ Failed to promote to stable: %v", err)
		UpdateCanaryErrmsg(db, config.CanaryDeploymentID, "promotion-failed")
		return
	}

	log.Printf("✅ Stable deployment updated to %s", config.ImageTag)

	// Cleanup canary - scale to 0
	cleanupYAML := modifyDeploymentForCanary(config.DeploymentYAML,
		config.CanaryDeployment, config.ImageTag, 0, "canary")
	if err := ApplyYAML(config.KubeconfigPath, config.ContextName, cleanupYAML); err != nil {
		log.Printf("⚠️ Failed to cleanup canary: %v", err)
		// Don't fail promotion if cleanup fails
	}

	// Update DB
	_, _ = db.Exec(context.Background(), `
        UPDATE deployments SET status='ready', canary_track='stable', updated_at=NOW()
        WHERE deployment_id=$1`, config.CanaryDeploymentID)

	log.Printf("🎉 PROMOTED! %s → PRODUCTION", config.ImageTag)
	svcName, svcPort, err := ExtractServiceNameAndPort(config.ServiceYAML)
	if err != nil {
		return
	}

	err = StartPortForward(
		config.KubeconfigPath,
		config.ContextName,
		config.Namespace,
		svcName,
		svcPort, // local port
		svcPort, // remote port (service port)
	)
}

// Helper function to update deployment errors
func UpdateCanaryErrmsg(db *pgxpool.Pool, deploymentID, errorType string) {
	errorMsg := fmt.Sprintf("Failed at %s", errorType)
	_, _ = db.Exec(context.Background(), `
        UPDATE deployments 
        SET status='failed', error_message=$1, failure_type='hard', updated_at=NOW()
        WHERE deployment_id=$2`, errorMsg, deploymentID)
}
