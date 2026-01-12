package controllers

import (
    "context"
    "encoding/base64"
    "log"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "gopkg.in/yaml.v3"
    "github.com/jackc/pgx/v5/pgxpool"
    services "github.com/Santhoshkumar044/MiniMon/client-go"
)

type CanaryController struct {
    DB *pgxpool.Pool
}

func NewCanaryController(db *pgxpool.Pool) *CanaryController {
    return &CanaryController{DB: db}
}

// Canary deployment - NEW ENDPOINT
func (cc *CanaryController) CanaryDeployWebhook(c *gin.Context) {
    var payload WebhookPayload
    if err := c.ShouldBindJSON(&payload); err != nil {
        log.Printf("❌ Invalid payload: %v\n", err)
        c.JSON(400, gin.H{"error": "Invalid payload"})
        return
    }

    // Get user's kubeconfig and context from database
    var configPath, contextName string
    err := cc.DB.QueryRow(context.Background(), 
        `SELECT u.config_path, p.context_name 
         FROM users u JOIN projects p ON u.user_id = p.user_id 
         WHERE u.user_id = $1 AND p.project_id = $2`, 
        payload.UserID, payload.ProjectID).Scan(&configPath, &contextName)
    if err != nil {
        log.Printf("❌ Failed to get user config: %v\n", err)
        c.JSON(404, gin.H{"error": "User or project not found"})
        return
    }

    // Decode YAML files
    secretYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Secret)
    serviceYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Service)
    deploymentYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Deployment)

    // Parse deployment YAML to extract metadata & replicas
    var deploymentObj struct {
        Metadata struct {
            Name      string `yaml:"name"`
            Namespace string `yaml:"namespace"`
        } `yaml:"metadata"`
        Spec struct {
            Replicas *int32 `yaml:"replicas"`
        } `yaml:"spec"`
    }
    yaml.Unmarshal(deploymentYAML, &deploymentObj)
    
    namespace := deploymentObj.Metadata.Namespace
    if namespace == "" { 
        namespace = "default" 
    }
    deploymentName := deploymentObj.Metadata.Name
    canaryDeploymentName := deploymentName + "-canary"  // 👈 FIX: Define this variable
    stableReplicas := int(*deploymentObj.Spec.Replicas)

    // CREATE CANARY DEPLOYMENT RECORD
    canaryID := uuid.New().String()
    _, err = cc.DB.Exec(context.Background(), `
        INSERT INTO deployments (
            deployment_id, project_id, commit_sha, image_tag, status, 
            namespace, deployment_name, deployment_type, canary_track,
            canary_target_replicas, canary_stage
        ) VALUES ($1, $2, $3, $4, 'pending', $5, $6, 'deploy', 'canary', $7, 0)`,
        canaryID, payload.ProjectID, payload.CommitSHA, payload.ImageTag, 
        namespace, canaryDeploymentName, stableReplicas)

    if err != nil {
        log.Printf("❌ Failed to create canary record: %v\n", err)
        c.JSON(500, gin.H{"error": "Database error"})
        return
    }

    log.Printf("🟡 CANARY STARTED: %s (stage=0, replicas=1/%d)", canaryID, stableReplicas)

    // Start canary pipeline in background
    go services.StartCanaryPipeline(cc.DB, services.CanaryPipelineConfig{
        KubeconfigPath:     configPath,
        ContextName:        contextName,
        Namespace:          namespace,
        DeploymentName:     deploymentName,
        CanaryDeploymentID: canaryID,
        CanaryDeployment:   canaryDeploymentName, 
        ImageTag:           payload.ImageTag,
        SecretYAML:         secretYAML,
        ServiceYAML:        serviceYAML,
        DeploymentYAML:     deploymentYAML,
        StableReplicas:     stableReplicas,
    })

    log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    log.Println("✅ CANARY DEPLOYMENT INITIATED")
    log.Printf(" Canary ID: %s\n", canaryID)
    log.Printf(" Traffic Split: 1/%d (17%%)\n", stableReplicas)
    log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

    c.JSON(200, gin.H{
        "status":        "canary_started",
        "canary_id":     canaryID,
        "stage":         0,
        "traffic_split": "1/" + string(rune(stableReplicas)) + " (17%)",
        "message":       "Canary deployed - monitoring stage 1...",
    })
}