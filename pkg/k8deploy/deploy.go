package k8deploy

import (
	"log"
	"fmt"
	yaml "github.com/alloy-go/Alloy-core/pkg/yamlApplier"
	"github.com/alloy-go/Alloy-core/pkg/types"
	"github.com/alloy-go/Alloy-core/pkg/status"
)

func DeployToKubernetes(config types.K8sDeployConfig) (*types.DeploymentResult, error) {
	results := &types.DeploymentResult{}

	// Update status to deploying
	Deploymentstatus.Update(config.DB, config.DeploymentID, "deploying", "", "", "")

	// Deploy Secret
	log.Println("📦 Deploying Secret...")
	if err := yaml.Apply(config.KubeconfigPath, config.ContextName, config.SecretYAML); err != nil {
		log.Printf("❌ Secret deployment failed: %v\n", err)
		results.Secret = "failed"
		Deploymentstatus.Update(config.DB, config.DeploymentID, "deploying", "failed", "", "")
	} else {
		log.Println("✅ Secret deployed")
		results.Secret = "success"
		Deploymentstatus.Update(config.DB, config.DeploymentID, "deploying", "success", "", "")
	}

	// Deploy Service
	log.Println("📦 Deploying Service...")
	if err := yaml.Apply(config.KubeconfigPath, config.ContextName, config.ServiceYAML); err != nil {
		log.Printf("❌ Service deployment failed: %v\n", err)
		results.Service = "failed"
		Deploymentstatus.Update(config.DB, config.DeploymentID, "deploying", results.Secret, "failed", "")
	} else {
		log.Println("✅ Service deployed")
		results.Service = "success"
		Deploymentstatus.Update(config.DB, config.DeploymentID, "deploying", results.Secret, "success", "")
	}

	// Deploy Deployment
	log.Println("📦 Deploying Deployment...")
	if err := yaml.Apply(config.KubeconfigPath, config.ContextName, config.DeploymentYAML); err != nil {
		log.Printf("❌ Deployment failed: %v\n", err)
		results.Deployment = "failed"
		Deploymentstatus.Update(config.DB, config.DeploymentID, "failed", results.Secret, results.Service, "failed")
		Deploymentstatus.UpdateError(config.DB, config.DeploymentID, err.Error())
		return results, fmt.Errorf("deployment failed: %w", err)
	}

	log.Println("✅ Deployment created")
	results.Deployment = "success"
	Deploymentstatus.Update(config.DB, config.DeploymentID, "deploying", results.Secret, results.Service, "success")

	// Start monitoring deployment status in background
	go monitorDeploymentStatus(config)

	return results, nil
}
