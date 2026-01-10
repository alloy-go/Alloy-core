package controllers

import (
    "context"
    "encoding/base64"
    "log"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "gopkg.in/yaml.v3"    
	"github.com/Santhoshkumar044/MiniMon/client-go"
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

func (wc *WebhookController)  DeployWebhook(c *gin.Context) {
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

    // Create deployment record in database
    deploymentID := uuid.New().String()
    _, err = wc.DB.Exec(context.Background(), `
        INSERT INTO deployments 
        (deployment_id, project_id, commit_sha, image_tag, status, namespace, deployment_name)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
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
