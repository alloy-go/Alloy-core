package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	services "github.com/minimon-cd/Alloy-core/client-go"
	"github.com/minimon-cd/Alloy-core/controllers"
	"github.com/minimon-cd/Alloy-core/utils"
)

func RegisterRoutes(r *gin.Engine, db *pgxpool.Pool) {
	api := r.Group("/api")

	// --------------------
	// utils
	// --------------------
	authService := utils.NewAuthService(db)
	projectService := utils.NewProjectService(db)
	userService := utils.NewUserService(db)
	
	// --------------------
	// Service metrics
	// --------------------

	kubeMetricsService := services.NewKubeMetricsService()
	doraService := services.NewDORAMetricsService(db)
	canaryMetricsService := services.NewCanaryMetricsService(db, kubeMetricsService) // for calculating canary metrics


	metricsService := services.NewMetricsService(
		db,
		kubeMetricsService,
		doraService,
		canaryMetricsService,
	)

	metricsController := controllers.NewMetricsController(metricsService)

	// --------------------
	// CONTROLLERS
	// --------------------
	initController := controllers.NewInitController(
		authService,
		projectService,
	)

	userController := controllers.NewUserController(userService)

	// NEW: Unified deployment controller
	deployController := controllers.NewDeploymentController(db,canaryMetricsService)


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
		project.DELETE("/:project_id",initController.DeleteProject)
	}

	users := api.Group("/users")
	{
		users.GET("/:user_id/view",userController.GetProfile)
		users.PUT("/:user_id/kubepath/edit",userController.UpdateKubeConfig)
	}

	//Metrics
	metrics := api.Group("/metrics/projects/:project_id")
	{
		metrics.GET("", metricsController.GetProjectMetrics)           // live
		metrics.GET("/v2", metricsController.GetProjectMetricsForCanary)  //canary metrics
		metrics.GET("/summary", metricsController.GetProjectMetricsSummary) // cached
		metrics.POST("refresh",metricsController.RefreshProjectMetrics) //Reload
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