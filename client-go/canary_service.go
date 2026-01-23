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
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🚀 PROMOTING CANARY TO PRODUCTION")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// STEP 1: Update stable deployment with new image
	log.Printf("📝 Updating stable deployment '%s' to image: %s", config.DeploymentName, config.ImageTag)  
	if err := PromoteStableByUpdatingExisting(
		config.KubeconfigPath,
		config.ContextName,
		config.Namespace,
		config.DeploymentName,
		config.ImageTag,
		config.StableReplicas,
	); err != nil {
		log.Printf("❌ Failed to promote to stable: %v", err)
		UpdateCanaryErrmsg(db, config.CanaryDeploymentID, "promotion-failed")
		return
	}

	log.Printf("✅ Stable deployment updated to %s", config.ImageTag)

	// STEP 2:WAIT for stable deployment to be ready BEFORE cleanup
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("⏳ Waiting for stable deployment '%s' to be ready...", config.DeploymentName)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	deadline := time.Now().Add(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	stableReady := false                                                  
	for time.Now().Before(deadline) {
		status, err := GetDeploymentStatus(
			config.KubeconfigPath,
			config.ContextName,
			config.Namespace,
			config.DeploymentName, 
		)

		if err != nil {
			log.Printf("⚠️ Status check failed: %v", err)
			<-ticker.C
			continue
		}

		log.Printf("📊 Stable deployment status: %s (%.1f min left)", status, time.Until(deadline).Minutes())

		if status == "ready" {
			log.Println("✅ Stable deployment is ready!")
			stableReady = true
			break
		}

		if status == "failed" {
			log.Println("❌ Stable deployment failed!")
			UpdateCanaryErrmsg(db, config.CanaryDeploymentID, "stable-promotion-failed")
			return
		}

		<-ticker.C
	}

	if !stableReady {
		log.Println("⏱️ Timeout waiting for stable deployment")
		UpdateCanaryErrmsg(db, config.CanaryDeploymentID, "stable-promotion-timeout")
		return
	}

	// STEP 3:NOW safe to cleanup canary (stable is fully ready)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("🗑️ Cleaning up canary deployment '%s'...", config.CanaryDeployment)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	cleanupYAML := modifyDeploymentForCanary(config.DeploymentYAML,
		config.CanaryDeployment, config.ImageTag, 0, "canary")
	if err := ApplyYAML(config.KubeconfigPath, config.ContextName, cleanupYAML); err != nil {
		log.Printf("⚠️ Failed to cleanup canary: %v", err)
		// Don't fail promotion if cleanup fails
	} else {
		log.Printf("✅ Canary deployment '%s' scaled to 0", config.CanaryDeployment) 
	}

	// STEP 4: Update database
	_, _ = db.Exec(context.Background(), `
        UPDATE deployments SET status='ready', canary_track='stable', updated_at=NOW()
        WHERE deployment_id=$1`, config.CanaryDeploymentID)

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("🎉 PROMOTION COMPLETE! %s → PRODUCTION", config.ImageTag)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// STEP 5:Port-forward to STABLE service (wait a bit for canary pods to fully terminate)
	log.Println("⏳ Waiting 10 seconds for canary pods to terminate...") 
	time.Sleep(10 * time.Second)                                          

	svcName, svcPort, err := ExtractServiceNameAndPort(config.ServiceYAML)
	if err != nil {
		log.Printf("❌ Failed to extract service info: %v", err)
		return
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🔌 STARTING PORT-FORWARD TO STABLE DEPLOYMENT")
	log.Printf("   Service: %s", svcName)
	log.Printf("   Port: %d", svcPort)
	log.Printf("   Target Deployment: %s (STABLE)", config.DeploymentName)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	err = StartPortForward(
		config.KubeconfigPath,
		config.ContextName,
		config.Namespace,
		svcName,
		svcPort,
		svcPort,
	)

	if err != nil {                                                       
		log.Printf("❌ Port-forward failed: %v", err)
	} else {
		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Printf("🎉 DEPLOYMENT COMPLETE - PORT %d IS ACTIVE", svcPort)
		log.Printf("   Access your app at: http://localhost:%d", svcPort)
		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}
}

// Helper function to update deployment errors
func UpdateCanaryErrmsg(db *pgxpool.Pool, deploymentID, errorType string) {
	errorMsg := fmt.Sprintf("Failed at %s", errorType)
	_, _ = db.Exec(context.Background(), `
        UPDATE deployments 
        SET status='failed', error_message=$1, failure_type='hard', updated_at=NOW()
        WHERE deployment_id=$2`, errorMsg, deploymentID)
}
