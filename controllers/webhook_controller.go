package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	services "github.com/Santhoshkumar044/MiniMon/client-go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"
)

type WebhookController struct {
	DB *pgxpool.Pool
}

func NewWebhookController(db *pgxpool.Pool) *WebhookController {
	return &WebhookController{DB: db}
}

type WebhookPayload struct {
	ProjectID string `json:"project_id"`
	UserID    string `json:"user_id"`
	ImageTag  string `json:"image_tag"`
	CommitSHA string `json:"commit_sha"`
	Files     struct {
		Secret     string `json:"secret"`
		Service    string `json:"service"`
		Deployment string `json:"deployment"`
	} `json:"files"`
}

func (wc *WebhookController) DeployWebhook(c *gin.Context) {
	var payload WebhookPayload

	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Printf("❌ Invalid payload: %v\n", err)
		c.JSON(400, gin.H{"error": "Invalid payload"})
		return
	}

	// Get user's kubeconfig and context from database
	var configPath, contextName string
	err := wc.DB.QueryRow(context.Background(), `
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

	// Decode YAML files
	secretYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Secret)
	serviceYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Service)
	deploymentYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Deployment)

	// Parse deployment YAML to extract namespace and name
	var deploymentObj struct {
		Metadata struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
	}
	yaml.Unmarshal(deploymentYAML, &deploymentObj)

	namespace := deploymentObj.Metadata.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// 🔑 PARSE SERVICE YAML
	var serviceObj struct {
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
		Spec struct {
			Selector map[string]string `yaml:"selector"`
			Ports    []struct {
				Port       int `yaml:"port"`
				TargetPort int `yaml:"targetPort"`
			} `yaml:"ports"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(serviceYAML, &serviceObj); err != nil {
		log.Printf("❌ Failed to parse service YAML: %v\n", err)
		c.JSON(400, gin.H{"error": "Invalid service YAML"})
		return
	}

	serviceName := serviceObj.Metadata.Name
	servicePort := 8080 // default
	if len(serviceObj.Spec.Ports) > 0 {
		servicePort = serviceObj.Spec.Ports[0].Port
	}

	// Extract app label from selector (usually "app: something")
	appLabel := ""
	if appName, exists := serviceObj.Spec.Selector["app"]; exists {
		appLabel = "app=" + appName
	} else {
		// Fallback: use first selector
		for key, value := range serviceObj.Spec.Selector {
			appLabel = fmt.Sprintf("%s=%s", key, value)
			break
		}
	}

	log.Printf("📋 Extracted from service.yaml:")
	log.Printf("   Service Name: %s\n", serviceName)
	log.Printf("   Service Port: %d\n", servicePort)
	log.Printf("   Label Selector: %s\n", appLabel)

	// Create deployment record in database
	deploymentID := uuid.New().String()
	_, err = wc.DB.Exec(context.Background(), `
       INSERT INTO deployments (
	   deployment_id, project_id, commit_sha, image_tag, status, 
	   namespace, deployment_name, deployment_type, canary_track, 
	   canary_target_replicas, canary_stage 
	   ) VALUES ($1, $2, $3, $4, 'pending', $5, $6, 'canary', 'canary', $7, 0)
    `, deploymentID, payload.ProjectID, payload.CommitSHA, payload.ImageTag, "pending", namespace, deploymentObj.Metadata.Name)

	if err != nil {
		log.Printf("❌ Failed to create deployment record: %v\n", err)
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("📡 DEPLOYING TO KUBERNETES")
	log.Printf("   Deployment ID: %s\n", deploymentID)
	log.Printf("   Project: %s\n", payload.ProjectID)
	log.Printf("   Image: %s\n", payload.ImageTag)
	log.Printf("   Commit: %s\n", payload.CommitSHA)
	log.Printf("   Context: %s\n", contextName)
	log.Printf("   Namespace: %s\n", namespace)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Deploy to Kubernetes
	deployConfig := services.K8sDeployConfig{
		KubeconfigPath: configPath,
		ContextName:    contextName,
		SecretYAML:     secretYAML,
		ServiceYAML:    serviceYAML,
		DeploymentYAML: deploymentYAML,
		DeploymentID:   deploymentID,
		Namespace:      namespace,
		DeploymentName: deploymentObj.Metadata.Name,
		ServiceName:    serviceName, // PASS SERVICE NAME
		ServicePort:    servicePort, // PASS PORT
		AppLabel:       appLabel,    // PASS LABEL
		DB:             wc.DB,
	}

	results, err := services.DeployToKubernetes(deployConfig)
	if err != nil {
		log.Printf("❌ Deployment failed: %v\n", err)
		c.JSON(500, gin.H{
			"status":        "failed",
			"deployment_id": deploymentID,
			"error":         err.Error(),
			"results":       results,
		})
		return
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("✅ DEPLOYMENT INITIATED")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	c.JSON(200, gin.H{
		"status":        "deploying",
		"deployment_id": deploymentID,
		"commit_sha":    payload.CommitSHA,
		"results":       results,
	})
}

// FAILED DEPLOY CHECK
func (wc *WebhookController) DeployWebhookFailTest(c *gin.Context) {
	var payload WebhookPayload

	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Printf("❌ Invalid payload: %v\n", err)
		c.JSON(400, gin.H{"error": "Invalid payload"})
		return
	}

	// Get user's kubeconfig and context from database
	var configPath, contextName string
	err := wc.DB.QueryRow(context.Background(), `
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

	// Decode YAML files
	secretYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Secret)
	serviceYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Service)
	deploymentYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Deployment)

	// 🔥 INJECT A BAD IMAGE TAG TO FORCE FAILURE
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("⚠️  FAIL TEST MODE: Using non-existent image")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Parse as generic map to preserve all fields
	var deploymentObj map[string]interface{}
	yaml.Unmarshal(deploymentYAML, &deploymentObj)

	// Navigate to image field
	spec := deploymentObj["spec"].(map[string]interface{})
	template := spec["template"].(map[string]interface{})
	podSpec := template["spec"].(map[string]interface{})
	containers := podSpec["containers"].([]interface{})
	container := containers[0].(map[string]interface{})

	originalImage := container["image"].(string)
	container["image"] = "rudraprasad792619/student-app:nonexistent-fail-tag"

	// Re-marshal with ALL fields preserved (apiVersion, kind, etc.)
	deploymentYAML, _ = yaml.Marshal(deploymentObj)

	log.Printf("   Original Image: %s\n", originalImage)
	log.Printf("   Failed Image: %s\n", container["image"])

	// Extract metadata
	metadata := deploymentObj["metadata"].(map[string]interface{})
	deploymentName := metadata["name"].(string)
	namespace, _ := metadata["namespace"].(string)
	if namespace == "" {
		namespace = "default"
	}

	// Parse service YAML
	var serviceObj struct {
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
		Spec struct {
			Selector map[string]string `yaml:"selector"`
			Ports    []struct {
				Port       int `yaml:"port"`
				TargetPort int `yaml:"targetPort"`
			} `yaml:"ports"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(serviceYAML, &serviceObj); err != nil {
		log.Printf("❌ Failed to parse service YAML: %v\n", err)
		c.JSON(400, gin.H{"error": "Invalid service YAML"})
		return
	}

	serviceName := serviceObj.Metadata.Name
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

	log.Printf("📋 Extracted from service.yaml:")
	log.Printf("   Service Name: %s\n", serviceName)
	log.Printf("   Service Port: %d\n", servicePort)
	log.Printf("   Label Selector: %s\n", appLabel)

	// Create deployment record
	deploymentID := uuid.New().String()
	_, err = wc.DB.Exec(context.Background(), `
        INSERT INTO deployments 
        (deployment_id, project_id, commit_sha, image_tag, status, namespace, deployment_name)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, deploymentID, payload.ProjectID, payload.CommitSHA, "nonexistent-fail-tag", "pending", namespace, deploymentName)

	if err != nil {
		log.Printf("❌ Failed to create deployment record: %v\n", err)
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("📡 DEPLOYING BAD IMAGE TO KUBERNETES")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Deploy to Kubernetes (with bad image)
	deployConfig := services.K8sDeployConfig{
		KubeconfigPath: configPath,
		ContextName:    contextName,
		SecretYAML:     secretYAML,
		ServiceYAML:    serviceYAML,
		DeploymentYAML: deploymentYAML,
		DeploymentID:   deploymentID,
		Namespace:      namespace,
		DeploymentName: deploymentName,
		ServiceName:    serviceName,
		ServicePort:    servicePort,
		AppLabel:       appLabel,
		DB:             wc.DB,
	}

	services.DeployToKubernetes(deployConfig)

	log.Println("✅ BAD DEPLOYMENT INITIATED (Will fail soon)")

	// 🔥 WAIT THEN TRIGGER AUTOMATIC ROLLBACK
	go func() {
		time.Sleep(30 * time.Second)

		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Println("🚨 DEPLOYMENT FAILED - TRIGGERING AUTO ROLLBACK")
		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		// Mark as failed directly in DB
		wc.DB.Exec(context.Background(), `
            UPDATE deployments 
            SET status = 'failed', 
                error_message = 'ImagePullBackOff: Image not found',
                updated_at = NOW()
            WHERE deployment_id = $1
        `, deploymentID)

		// Trigger rollback
		rollbackResult, err := services.ExecuteRollback(wc.DB, services.RollbackRequest{
			ProjectID:          payload.ProjectID,
			UserID:             payload.UserID,
			FailedDeploymentID: deploymentID,
		})

		if err != nil {
			log.Printf("❌ Rollback failed: %v\n", err)
			return
		}

		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		log.Println("✅ ROLLBACK INITIATED")
		log.Printf("   Rollback Deployment ID: %s\n", rollbackResult.RollbackDeploymentID)
		log.Printf("   Rolling back to image: %s\n", rollbackResult.TargetImageTag)
		log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}()

	c.JSON(200, gin.H{
		"status":        "deploying_bad_image",
		"deployment_id": deploymentID,
		"message":       "Deploying bad image - will auto-rollback in 30 seconds",
	})
}
