package project

import (
	"net/http"
	
	"github.com/gin-gonic/gin"
)

type Handler struct{
	Service  *Service
}

func NewHandler (project *Service) *Handler {
	return &Handler{
		Service: project,
	}
}

func (pr *Handler) CreateProject(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		UserID         string `json:"user_id"`
		ProjectName    string `json:"project_name"`
		DeploymentType string `json:"deployment_type"`
		ContextName    string `json:"context_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := pr.Service.CreateProject(
		ctx,
		req.UserID,
		req.ProjectName,
		req.DeploymentType,
		req.ContextName,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "project creation failed",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "project created successfully",
	})
}

//Dellete project
func (pr *Handler) DeleteProject(c *gin.Context) {
	ctx := c.Request.Context()

	projectID := c.Param("project_id")

	var req struct {
		UserID string `json:"user_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == "" {
		c.JSON(400, gin.H{
			"error": "user_id is required",
		})
		return
	}

	err := pr.Service.DeleteProject(
		ctx,
		projectID,
		req.UserID,
	)

	if err != nil {
		c.JSON(404, gin.H{
			"error": "project not found or not authorized",
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "project deleted successfully",
	})
}

func (pr *Handler) GetUserProjects(c *gin.Context) {
	ctx := c.Request.Context()

	userID := c.Param("user_id") // get user ID from URL path
	if userID == "" {
		c.JSON(400, gin.H{"error": "user_id is required"})
		return
	}

	projects, err := pr.Service.GetProjectsWithDeploymentInfo(ctx, userID) // use existing service
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to fetch projects"})
		return
	}

	c.JSON(200, gin.H{"projects": projects})
}