package user

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Routes(r *gin.Engine,db *pgxpool.Pool) {
	api := r.Group("/user")

	userHandler := NewUserHandler(NewService(db))

	api.GET("/:user_id",userHandler.ProfileHandler)
	api.PUT("/:user_id/kubepath/edit",userHandler.Kubeconfig)
}