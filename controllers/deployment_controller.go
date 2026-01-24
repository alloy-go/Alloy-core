package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	services "github.com/minimon-cd/Alloy-core/client-go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"
)

type DeploymentController struct {
	DB *pgxpool.Pool
}

func NewDeploymentController(db *pgxpool.Pool) *DeploymentController {
	return &DeploymentController{DB: db}
}

// DeploymentStrategy defines how deployments should be executed
type DeploymentStrategy string

const (
	StrategyRollout DeploymentStrategy = "rollout" // Default: direct rolling update
	StrategyCanary  DeploymentStrategy = "canary"  // Gradual canary with stages
	StrategyAuto    DeploymentStrategy = "auto"    // Auto-detect based on project history
)

type UnifiedWebhookPayload struct {
	ProjectID string             `json:"project_id"`
	UserID    string             `json:"user_id"`
	ImageTag  string             `json:"image_tag"`
	CommitSHA string             `json:"commit_sha"`
	Strategy  DeploymentStrategy `json:"strategy,omitempty"` // Optional: defaults to "auto"
	Files     struct {
		Secret     string `json:"secret"`
		Service    string `json:"service"`
		Deployment string `json:"deployment"`
	} `json:"files"`
}

// 🎯 MAIN UNIFIED DEPLOYMENT ENDPOINT
func (dc *DeploymentController) Deploy(c *gin.Context) {
	var payload UnifiedWebhookPayload

	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Printf("❌ Invalid payload: %v\n", err)
		c.JSON(400, gin.H{"error": "Invalid payload"})
		return
	}

	// 1. Determine deployment strategy
	strategy := dc.determineStrategy(payload)
	log.Printf("📋 Selected strategy: %s", strategy)

	// 2. Get user config
	var configPath, contextName string
	err := dc.DB.QueryRow(context.Background(), `
        SELECT u.config_path, p.context_name 
        FROM users u 
        JOIN projects p ON u.user_id = p.user_id 
        WHERE u.user_id = $1 AND p.project_id = $2
    `, payload.UserID, payload.ProjectID).Scan(&configPath, &contextName)

	if err != nil {
		log.Printf("❌ Failed to get user config: %v\n", err)
		c.JSON(404, gin.H{"error": "User or project not found"})
		return
	}

	// 3. Decode YAML files
	secretYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Secret)
	serviceYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Service)
	deploymentYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Deployment)

	// 4. Parse deployment metadata
	deploymentMeta := dc.parseDeploymentMetadata(deploymentYAML)
	serviceMeta := dc.parseServiceMetadata(serviceYAML)

	// 5. Route to appropriate deployment strategy
	switch strategy {
	case StrategyCanary:
		dc.executeCanaryDeployment(c, payload, configPath, contextName, 
			secretYAML, serviceYAML, deploymentYAML, deploymentMeta, serviceMeta)
	
	case StrategyRollout:
		dc.executeRolloutDeployment(c, payload, configPath, contextName, 
			secretYAML, serviceYAML, deploymentYAML, deploymentMeta, serviceMeta)
	
	default:
		c.JSON(400, gin.H{"error": "Unknown deployment strategy"})
	}
}

// 🧠 Smart strategy determination
func (dc *DeploymentController) determineStrategy(payload UnifiedWebhookPayload) DeploymentStrategy {
	// 1. If explicitly specified, use that
	if payload.Strategy != "" && payload.Strategy != StrategyAuto {
		return payload.Strategy
	}

	// 2. Check if this is first deployment (no previous deployments)
	var deploymentCount int
	err := dc.DB.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM deployments 
		WHERE project_id = $1 AND status = 'ready'
	`, payload.ProjectID).Scan(&deploymentCount)

	if err != nil || deploymentCount == 0 {
		log.Println("🆕 First deployment detected → using ROLLOUT strategy")
		return StrategyRollout
	}

	// 3. Check project configuration for default strategy
	var defaultStrategy string
	err = dc.DB.QueryRow(context.Background(), `
		SELECT deployment_strategy FROM projects 
		WHERE project_id = $1
	`, payload.ProjectID).Scan(&defaultStrategy)

	if err == nil && defaultStrategy != "" {
		log.Printf("⚙️  Using project default strategy: %s", defaultStrategy)
		return DeploymentStrategy(defaultStrategy)
	}

	// 4. Default to canary for subsequent deployments (safer)
	log.Println("🔄 Subsequent deployment → using CANARY strategy (safe default)")
	return StrategyCanary
}

// Metadata structures
type DeploymentMetadata struct {
	Name      string
	Namespace string
	Replicas  int
}

type ServiceMetadata struct {
	Name     string
	Port     int
	Selector map[string]string
	AppLabel string
}

func (dc *DeploymentController) parseDeploymentMetadata(yamlContent []byte) DeploymentMetadata {
	var deploymentObj struct {
		Metadata struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			Replicas *int32 `yaml:"replicas"`
		} `yaml:"spec"`
	}
	yaml.Unmarshal(yamlContent, &deploymentObj)

	namespace := deploymentObj.Metadata.Namespace
	if namespace == "" {
		namespace = "default"
	}

	replicas := 1
	if deploymentObj.Spec.Replicas != nil {
		replicas = int(*deploymentObj.Spec.Replicas)
	}

	return DeploymentMetadata{
		Name:      deploymentObj.Metadata.Name,
		Namespace: namespace,
		Replicas:  replicas,
	}
}

func (dc *DeploymentController) parseServiceMetadata(yamlContent []byte) ServiceMetadata {
	var serviceObj struct {
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
		Spec struct {
			Selector map[string]string `yaml:"selector"`
			Ports    []struct {
				Port int `yaml:"port"`
			} `yaml:"ports"`
		} `yaml:"spec"`
	}
	yaml.Unmarshal(yamlContent, &serviceObj)

	servicePort := 8080
	if len(serviceObj.Spec.Ports) > 0 {
		servicePort = serviceObj.Spec.Ports[0].Port
	}

	appLabel := ""
	if appName, exists := serviceObj.Spec.Selector["app"]; exists {
		appLabel = "app=" + appName
	} else {
		for key, value := range serviceObj.Spec.Selector {
			appLabel = fmt.Sprintf("%s=%s", key, value)
			break
		}
	}

	return ServiceMetadata{
		Name:     serviceObj.Metadata.Name,
		Port:     servicePort,
		Selector: serviceObj.Spec.Selector,
		AppLabel: appLabel,
	}
}

// 🚀 Execute rollout deployment
func (dc *DeploymentController) executeRolloutDeployment(
	c *gin.Context,
	payload UnifiedWebhookPayload,
	configPath, contextName string,
	secretYAML, serviceYAML, deploymentYAML []byte,
	deploymentMeta DeploymentMetadata,
	serviceMeta ServiceMetadata,
) {
	deploymentID := uuid.New().String()
	
	_, err := dc.DB.Exec(context.Background(), `
		INSERT INTO deployments (
			deployment_id, project_id, commit_sha, image_tag, status, 
			namespace, deployment_name, deployment_type, canary_track
		) VALUES ($1, $2, $3, $4, 'pending', $5, $6, 'rollout', 'stable')
	`, deploymentID, payload.ProjectID, payload.CommitSHA, payload.ImageTag,
		deploymentMeta.Namespace, deploymentMeta.Name)

	if err != nil {
		log.Printf("❌ Failed to create deployment record: %v\n", err)
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🚀 ROLLOUT DEPLOYMENT")
	log.Printf("   Deployment ID: %s", deploymentID)
	log.Printf("   Strategy: Direct Rolling Update")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	deployConfig := services.K8sDeployConfig{
		KubeconfigPath: configPath,
		ContextName:    contextName,
		SecretYAML:     secretYAML,
		ServiceYAML:    serviceYAML,
		DeploymentYAML: deploymentYAML,
		DeploymentID:   deploymentID,
		Namespace:      deploymentMeta.Namespace,
		DeploymentName: deploymentMeta.Name,
		ServiceName:    serviceMeta.Name,
		ServicePort:    serviceMeta.Port,
		AppLabel:       serviceMeta.AppLabel,
		DB:             dc.DB,
	}

	results, err := services.DeployToKubernetes(deployConfig)
	if err != nil {
		log.Printf("❌ Deployment failed: %v\n", err)
		c.JSON(500, gin.H{
			"status":        "failed",
			"deployment_id": deploymentID,
			"error":         err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":        "deploying",
		"deployment_id": deploymentID,
		"strategy":      "rollout",
		"commit_sha":    payload.CommitSHA,
		"results":       results,
	})
}

// 🟡 Execute canary deployment
func (dc *DeploymentController) executeCanaryDeployment(
	c *gin.Context,
	payload UnifiedWebhookPayload,
	configPath, contextName string,
	secretYAML, serviceYAML, deploymentYAML []byte,
	deploymentMeta DeploymentMetadata,
	serviceMeta ServiceMetadata,
) {
	canaryID := uuid.New().String()
	canaryDeploymentName := deploymentMeta.Name + "-canary"

	_, err := dc.DB.Exec(context.Background(), `
		INSERT INTO deployments (
			deployment_id, project_id, commit_sha, image_tag, status, 
			namespace, deployment_name, deployment_type, canary_track,
			canary_target_replicas, canary_stage
		) VALUES ($1, $2, $3, $4, 'pending', $5, $6, 'canary', 'canary', $7, 0)
	`, canaryID, payload.ProjectID, payload.CommitSHA, payload.ImageTag,
		deploymentMeta.Namespace, canaryDeploymentName, deploymentMeta.Replicas)

	if err != nil {
		log.Printf("❌ Failed to create canary record: %v\n", err)
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🟡 CANARY DEPLOYMENT")
	log.Printf("   Canary ID: %s", canaryID)
	log.Printf("   Strategy: Progressive Rollout (3 stages)")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	go services.StartCanaryPipeline(dc.DB, services.CanaryPipelineConfig{
		KubeconfigPath:     configPath,
		ContextName:        contextName,
		Namespace:          deploymentMeta.Namespace,
		DeploymentName:     deploymentMeta.Name,
		CanaryDeploymentID: canaryID,
		CanaryDeployment:   canaryDeploymentName,
		ImageTag:           payload.ImageTag,
		SecretYAML:         secretYAML,
		ServiceYAML:        serviceYAML,
		DeploymentYAML:     deploymentYAML,
		StableReplicas:     deploymentMeta.Replicas,
	})

	c.JSON(200, gin.H{
		"status":        "canary_started",
		"deployment_id": canaryID,
		"strategy":      "canary",
		"stage":         0,
		"message":       "Canary deployment initiated - monitoring stage 1...",
	})
}