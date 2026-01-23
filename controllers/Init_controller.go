package controllers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"context"
	"log"

	"github.com/Santhoshkumar044/MiniMon/utils"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InitController struct {
	AuthService    *utils.AuthService
	ProjectService *utils.ProjectService
}

func NewInitController(
	auth *utils.AuthService,
	project *utils.ProjectService,
) *InitController {
	return &InitController{
		AuthService:    auth,
		ProjectService: project,
	}
}

// --------------------
// SIGNUP (UI)
// POST /auth/signup
// --------------------
func (ic *InitController) Signup(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		Username   string `json:"username"`
		Email      string `json:"email"`
		Password   string `json:"password"`
		Token      string `json:"token"`
		ConfigPath string `json:"config_path"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := ic.AuthService.Signup(
		ctx,
		req.Username,
		req.Email,
		req.Password,
		req.Token,
		req.ConfigPath,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// 23505 is unique constraint violation
			if pgErr.Code == "23505" {
				if strings.Contains(pgErr.ConstraintName, "email") ||
					strings.Contains(pgErr.Detail, "email") {
					c.JSON(http.StatusConflict, gin.H{
						"error": "Email already exists",
					})
					return
				}
				if strings.Contains(pgErr.ConstraintName, "username") ||
					strings.Contains(pgErr.Detail, "username") {
					c.JSON(http.StatusConflict, gin.H{
						"error": "Username already exists",
					})
					return
				}
				c.JSON(http.StatusConflict, gin.H{
					"error": "Record already exists",
				})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Signup failed",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user_id": userID,
	})
}

// --------------------
// LOGIN (UI)
// POST /auth/login
// --------------------
func (ic *InitController) Login(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := ic.AuthService.Login(
		ctx,
		req.Username,
		req.Password,
	)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid credentials",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
	})
}

// --------------------
// CREATE PROJECT (UI)
// POST /projects
// --------------------
func (ic *InitController) CreateProject(c *gin.Context) {
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

	err := ic.ProjectService.CreateProject(
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

// COMBINED INIT (CLI)
// POST /init

func (ic *InitController) Init(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		Mode string `json:"mode"` // signup | login

		// auth
		Username   string `json:"username"`
		Email      string `json:"email"`
		Password   string `json:"password"`
		Token      string `json:"token"`
		ConfigPath string `json:"config_path"`

		// project
		ProjectName    string `json:"project_name"`
		DeploymentType string `json:"deployment_type"`
		ContextName    string `json:"context_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var (
		userID string
		err    error
	)

	// Decide flow for CLI
	if req.Mode == "signup" {
		userID, err = ic.AuthService.Signup(
			ctx,
			req.Username,
			req.Email,
			req.Password,
			req.Token,
			req.ConfigPath,
		)
	} else {
		userID, err = ic.AuthService.Login(
			ctx,
			req.Username,
			req.Password,
		)
	}

	if err != nil {
		c.JSON(401, gin.H{"error": "authentication failed"})
		return
	}

	err = ic.ProjectService.CreateProject(
		ctx,
		userID,
		req.ProjectName,
		req.DeploymentType,
		req.ContextName,
	)

	if err != nil {
		c.JSON(500, gin.H{"error": "project creation failed"})
		return
	}

	c.JSON(201, gin.H{
		"message": "minimon initialized successfully",
		"user_id": userID,
	})
}

func (ic *InitController) GetUserProjects(c *gin.Context) {
	ctx := c.Request.Context()

	userID := c.Param("user_id") // get user ID from URL path
	if userID == "" {
		c.JSON(400, gin.H{"error": "user_id is required"})
		return
	}

	projects, err := ic.ProjectService.GetProjectsByUser(ctx, userID) // use existing service
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to fetch projects"})
		return
	}

	c.JSON(200, gin.H{"projects": projects})
}

type UpdateStrategyRequest struct {
	Strategy string `json:"strategy" binding:"required,oneof=auto rollout canary"`
}

// Add this helper to ProjectService if you want
type ProjectService struct {
	DB *pgxpool.Pool
}

// UpdateDeploymentStrategy allows users to set their preferred deployment strategy
func (ic *InitController) UpdateDeploymentStrategy(c *gin.Context) {
	projectID := c.Param("project_id")

	var req UpdateStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid strategy. Must be one of: auto, rollout, canary",
		})
		return
	}

	// Update project strategy
	result, err := ic.ProjectService.DB.Exec(context.Background(), `
		UPDATE projects 
		SET deployment_strategy = $1, updated_at = NOW()
		WHERE project_id = $2
	`, req.Strategy, projectID)

	if err != nil {
		log.Printf("❌ Failed to update deployment strategy: %v", err)
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(404, gin.H{"error": "Project not found"})
		return
	}

	log.Printf("✅ Project %s deployment strategy updated to: %s", projectID, req.Strategy)

	c.JSON(200, gin.H{
		"project_id": projectID,
		"strategy":   req.Strategy,
		"message":    getStrategyDescription(req.Strategy),
	})
}

func getStrategyDescription(strategy string) string {
	descriptions := map[string]string{
		"auto":    "Automatic strategy selection (rollout for first deploy, canary for updates)",
		"rollout": "Direct rolling updates for all deployments (faster, higher risk)",
		"canary":  "Progressive canary deployments for all updates (safer, slower)",
	}
	return descriptions[strategy]
}

func (ps *ProjectService) GetProjectStrategy(projectID string) (string, error) {
	var strategy string
	err := ps.DB.QueryRow(context.Background(), `
		SELECT deployment_strategy FROM projects WHERE project_id = $1
	`, projectID).Scan(&strategy)

	if err != nil {
		return "auto", err
	}
	return strategy, nil
}
