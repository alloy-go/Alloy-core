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

	// NEW: Unified deployment controller
	deployController := controllers.NewDeploymentController(db)

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
		project.GET("/:user_id",initController.GetUserProjects)
	}

	// CLI route (minimon init)
	api.POST("/init", initController.Init)

	// --------------------
	// WEBHOOK ROUTES
	// --------------------
	webhookRouter := api.Group("/webhook")
	{
		// MAIN ENDPOINT - Auto-detects strategy
		webhookRouter.POST("/deploy", deployController.Deploy)
		
		// Legacy endpoints (optional - for backward compatibility)
		webhookRouter.POST("/deploy/rollout", deployController.DeployRollout)
		webhookRouter.POST("/deploy/canary", deployController.DeployCanary)
		
		// Test endpoint
		webhookRouter.POST("/deploy-fail-test", deployController.DeployFailTest)
	}

	// --------------------
	// DEPLOYMENT STATUS & MANAGEMENT
	// --------------------
	deploymentsRouter := api.Group("/deployments")
	{
		deploymentsRouter.GET("/:deployment_id", deployController.GetDeploymentStatus)
		deploymentsRouter.GET("/project/:project_id", deployController.GetProjectDeployments)
		deploymentsRouter.POST("/:deployment_id/promote", deployController.PromoteCanary)
		deploymentsRouter.POST("/:deployment_id/abort", deployController.AbortCanary)
	}
}