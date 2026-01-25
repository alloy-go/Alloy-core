package controllers

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Legacy endpoint wrappers for backward compatibility
func (dc *DeploymentController) DeployRollout(c *gin.Context) {
	var payload UnifiedWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(400, gin.H{"error": "Invalid payload"})
		return
	}
	payload.Strategy = StrategyRollout
	dc.Deploy(c)
}

func (dc *DeploymentController) DeployCanary(c *gin.Context) {
	var payload UnifiedWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(400, gin.H{"error": "Invalid payload"})
		return
	}
	payload.Strategy = StrategyCanary
	dc.Deploy(c)
}

// Get deployment status
func (dc *DeploymentController) GetDeploymentStatus(c *gin.Context) {
	deploymentID := c.Param("deployment_id")

	var deployment struct {
		ID             string `json:"deployment_id"`
		ProjectID      string `json:"project_id"`
		Status         string `json:"status"`
		DeploymentType string `json:"deployment_type"`
		CanaryTrack    string `json:"canary_track"`
		CanaryStage    int    `json:"canary_stage"`
		ImageTag       string `json:"image_tag"`
		CommitSHA      string `json:"commit_sha"`
		ErrorMessage   string `json:"error_message"`
	}

	err := dc.DB.QueryRow(context.Background(), `
		SELECT deployment_id, project_id, status, deployment_type, 
			   canary_track, canary_stage, image_tag, commit_sha,
			   COALESCE(error_message, '') as error_message
		FROM deployments 
		WHERE deployment_id = $1
	`, deploymentID).Scan(
		&deployment.ID, &deployment.ProjectID, &deployment.Status,
		&deployment.DeploymentType, &deployment.CanaryTrack, &deployment.CanaryStage,
		&deployment.ImageTag, &deployment.CommitSHA, &deployment.ErrorMessage,
	)

	if err != nil {
		c.JSON(404, gin.H{"error": "Deployment not found"})
		return
	}

	c.JSON(200, deployment)
}

// Get all deployments for a project
func (dc *DeploymentController) GetProjectDeployments(c *gin.Context) {
	projectID := c.Param("project_id")

	rows, err := dc.DB.Query(context.Background(), `
		SELECT deployment_id, status, deployment_type, canary_track, 
			   image_tag, commit_sha, created_at
		FROM deployments 
		WHERE project_id = $1 
		ORDER BY created_at DESC 
		LIMIT 50
	`, projectID)

	if err != nil {
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	deployments := []map[string]interface{}{}
	for rows.Next() {
		var d struct {
			ID             string
			Status         string
			DeploymentType string
			CanaryTrack    string
			ImageTag       string
			CommitSHA      string
			CreatedAt      time.Time
		}
		rows.Scan(&d.ID, &d.Status, &d.DeploymentType, &d.CanaryTrack,
			&d.ImageTag, &d.CommitSHA, &d.CreatedAt)

		deployments = append(deployments, map[string]interface{}{
			"deployment_id":   d.ID,
			"status":          d.Status,
			"deployment_type": d.DeploymentType,
			"canary_track":    d.CanaryTrack,
			"image_tag":       d.ImageTag,
			"commit_sha":      d.CommitSHA,
			"created_at":      d.CreatedAt,
		})
	}

	c.JSON(200, gin.H{
		"project_id":  projectID,
		"deployments": deployments,
	})
}

// Manually promote canary to stable and helps to calculate canary metrics
func (dc *DeploymentController) PromoteCanary(c *gin.Context) {
	deploymentID := c.Param("deployment_id")

	var status string
	var projectID string

	err := dc.DB.QueryRow(context.Background(), `
		SELECT status, project_id
		FROM deployments
		WHERE deployment_id = $1
		  AND deployment_type = 'canary'
	`, deploymentID).Scan(&status, &projectID)

	if err != nil {
		c.JSON(404, gin.H{"error": "Canary deployment not found"})
		return
	}

	if status != "stage_passed" && status != "analyzing" {
		c.JSON(400, gin.H{"error": "Canary not ready for promotion"})
		return
	}

	// ✅ 1. Finalize canary metrics FIRST (historical snapshot)
	if dc.CanaryMetricsService != nil {
		err = dc.CanaryMetricsService.FinalizeCanaryResult(
			context.Background(),
			deploymentID,
			"passed",
			"promote",
		)
		if err != nil {
			log.Printf("⚠️ Failed to finalize canary metrics: %v", err)
		} else {
			log.Printf("✅ Canary metrics finalized")
		}
	}

	// ✅ 2. Promote canary → stable rollout
	_, err = dc.DB.Exec(context.Background(), `
		UPDATE deployments stable
		SET
			image_tag  = canary.image_tag,
			commit_sha = canary.commit_sha,
			updated_at = NOW(),
			status     = 'ready'
		FROM deployments canary
		WHERE
			stable.project_id = canary.project_id
			AND stable.deployment_type = 'rollout'
			AND stable.canary_track = 'stable'
			AND canary.deployment_id = $1
	`, deploymentID)

	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to initiate promotion"})
		return
	}

	// ✅ 3. Mark canary as completed
	_, _ = dc.DB.Exec(context.Background(), `
		UPDATE deployments
		SET status = 'promoted', updated_at = NOW()
		WHERE deployment_id = $1
	`, deploymentID)

	log.Printf("🚀 Canary %s successfully promoted to production", deploymentID)

	c.JSON(200, gin.H{
		"status":  "promoted",
		"message": "Canary successfully promoted to production",
	})
}


// Update AbortCanary method
func (dc *DeploymentController) AbortCanary(c *gin.Context) {
	deploymentID := c.Param("deployment_id")

	// Finalize canary metrics BEFORE aborting
	if dc.CanaryMetricsService != nil {
		err := dc.CanaryMetricsService.FinalizeCanaryResult(
			context.Background(),
			deploymentID,
			"failed",
			"abort",
		)
		if err != nil {
			log.Printf("⚠️  Warning: failed to finalize canary metrics: %v", err)
		} else {
			log.Printf("✅ Canary metrics finalized as 'failed/aborted'")
		}
	}

	_, err := dc.DB.Exec(context.Background(), `
		UPDATE deployments 
		SET status = 'aborted', updated_at = NOW()
		WHERE deployment_id = $1 AND deployment_type = 'canary'
	`, deploymentID)

	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to abort deployment"})
		return
	}

	log.Printf("🛑 Canary deployment %s aborted by user", deploymentID)
	c.JSON(200, gin.H{
		"status":  "aborted",
		"message": "Canary deployment aborted successfully",
	})
}

// Test endpoint for failure scenarios
func (dc *DeploymentController) DeployFailTest(c *gin.Context) {
	var payload UnifiedWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(400, gin.H{"error": "Invalid payload"})
		return
	}

	// Force rollout strategy with bad image for testing
	payload.Strategy = StrategyRollout

	// You can add image manipulation logic here similar to your original
	// DeployWebhookFailTest implementation

	dc.Deploy(c)
}
