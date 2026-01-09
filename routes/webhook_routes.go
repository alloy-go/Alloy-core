package routes

import (
	"github.com/Santhoshkumar044/MiniMon/controllers"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func WebhookRoutes(r *gin.Engine, db *pgxpool.Pool) {
	api := r.Group("/api")

	webhookRouter := api.Group("/webhook")
	{
		webhookRouter.POST("/deploy", controllers.DeployWebhook)
	}
}
