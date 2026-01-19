package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Santhoshkumar044/MiniMon/controllers"
	"github.com/Santhoshkumar044/MiniMon/utils"
)

func RegisterRoutes(r *gin.Engine, db *pgxpool.Pool) {
	api := r.Group("/api")

	// --------------------
	// utils
	// --------------------
	authService := utils.NewAuthService(db)
	projectService := utils.NewProjectService(db)

	// --------------------
	// CONTROLLERS
	// --------------------
	initController := controllers.NewInitController(
		authService,
		projectService,
	)

	deployController := controllers.NewWebhookController(db)
	canaryController := controllers.NewCanaryController(db)

	// --------------------
	// INIT ROUTES
	// --------------------

	// UI routes
	auth := api.Group("/auth")
	{
		auth.POST("/signup", initController.Signup)
		auth.POST("/login", initController.Login)
	}

	project := api.Group("/projects")
	{
		project.POST("", initController.CreateProject)
	}

	// CLI route (minimon init)
	api.POST("/init", initController.Init)

	// --------------------
	// WEBHOOK ROUTES
	// --------------------
	webhookRouter := api.Group("/webhook")
	{
		webhookRouter.POST("/deploy", deployController.DeployWebhook)
		webhookRouter.POST("/deploy/canary", canaryController.CanaryDeployWebhook)
		webhookRouter.POST("/deploy-fail-test", deployController.DeployWebhookFailTest)
	}
}
