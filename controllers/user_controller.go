package controllers

import (
	"github.com/alloy-go/Alloy-core/utils"
	"github.com/gin-gonic/gin"
)

type UserController struct {
	UserService *utils.UserService
}

func NewUserController(userService *utils.UserService) *UserController {
	return &UserController{
		UserService: userService,
	}
}

// ----------------------
// GET USER PROFILE
// GET /users/:user_id/profile
// ----------------------
func (uc *UserController) GetProfile(c *gin.Context) {
	ctx := c.Request.Context()

	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(400, gin.H{"error": "user_id is required"})
		return
	}

	user, err := uc.UserService.GetUserByID(ctx, userID)
	if err != nil {
		c.JSON(404, gin.H{
			"error": "user not found",
		})
		return
	}

	c.JSON(200, gin.H{
		"user": user,
	})
}

//edit kube config path
func (uc *UserController) UpdateKubeConfig(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.Param("user_id")

	var req struct {
		ConfigPath string `json:"config_path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": "config_path is required",
		})
		return
	}

	err := uc.UserService.UpdateKubeConfigPath(
		ctx,
		userID,
		req.ConfigPath,
	)

	if err != nil {
		c.JSON(500, gin.H{
			"error": "failed to update kubeconfig",
		})
		return
	}

	c.JSON(200, gin.H{
		"message":     "kubeconfig updated successfully",
		"config_path": req.ConfigPath,
	})
}
