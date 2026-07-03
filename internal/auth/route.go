package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"	
)

func AuthRoutes(r *gin.Engine,db *pgxpool.Pool){
	api := r.Group("/auth")

	service := NewService(db)
	handler := NewHandler(service) 

	api.POST("/login",handler.Login)
	api.POST("/signup",handler.Signup)
}