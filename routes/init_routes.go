package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Santhoshkumar044/MiniMon/controllers"
)

func RegisterRoutes(r *gin.Engine, db *pgxpool.Pool) {
	api := r.Group("/api")

	initController := controllers.NewInitController(db)

	init := api.Group("/init")
	{
		init.POST("/signup", initController.SignupAndCreateProject)
		init.POST("/login", initController.LoginAndCreateProject)
	}

	webhookRouter := api.Group("/webhook")
	{
		webhookRouter.POST("/deploy", controllers.DeployWebhook)
	}
}
