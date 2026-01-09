package controllers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type InitController struct {
	DB *pgxpool.Pool
}

func NewInitController(db *pgxpool.Pool) *InitController {
	return &InitController{DB: db}
}

func (ic *InitController) SignupAndCreateProject(c *gin.Context) {
	ctx := context.Background()

	var req struct {
		Username       string `json:"username"`
		Email          string `json:"email"`
		Password       string `json:"password"`
		Token          string `json:"token"`
		ConfigPath     string `json:"config_path"`
		ProjectName    string `json:"project_name"`
		DeploymentType string `json:"deployment_type"`
		ContextName    string `json:"context_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 12)

	var userID string
	err := ic.DB.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, token, config_path)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING user_id
	`,
		req.Username, req.Email, string(hash), req.Token, req.ConfigPath,
	).Scan(&userID)

	if err != nil {
		c.JSON(500, gin.H{"error": "user creation failed"})
		return
	}

	_, err = ic.DB.Exec(ctx, `
		INSERT INTO projects (user_id, project_name, deployment_type, context_name)
		VALUES ($1,$2,$3,$4)
	`,
		userID, req.ProjectName, req.DeploymentType, req.ContextName,
	)

	if err != nil {
		c.JSON(500, gin.H{"error": "project creation failed"})
		return
	}

	c.JSON(201, gin.H{
		"message": "Init completed successfully",
		"user_id": userID,
	})
}

func (ic *InitController) LoginAndCreateProject(c *gin.Context) {
	ctx := context.Background()

	var req struct {
		Username       string `json:"username"`
		Password       string `json:"password"`
		ProjectName    string `json:"project_name"`
		DeploymentType string `json:"deployment_type"`
		ContextName    string `json:"context_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var userID, hash string
	err := ic.DB.QueryRow(ctx,
		`SELECT user_id, password_hash FROM users WHERE username=$1`,
		req.Username,
	).Scan(&userID, &hash)

	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}

	_, err = ic.DB.Exec(ctx, `
		INSERT INTO projects (user_id, project_name, deployment_type, context_name)
		VALUES ($1,$2,$3,$4)
	`, userID, req.ProjectName, req.DeploymentType, req.ContextName)

	if err != nil {
		c.JSON(500, gin.H{"error": "project creation failed"})
		return
	}

	c.JSON(200, gin.H{
		"message": "Init completed",
		"user_id": userID,
	})
}
