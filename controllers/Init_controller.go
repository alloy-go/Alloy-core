package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Santhoshkumar044/MiniMon/utils"
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

//
// --------------------
// SIGNUP (UI)
// POST /auth/signup
// --------------------
//
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "signup failed",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user_id": userID,
	})
}

//
// --------------------
// LOGIN (UI)
// POST /auth/login
// --------------------
//
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

//
// --------------------
// CREATE PROJECT (UI)
// POST /projects
// --------------------
//
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
