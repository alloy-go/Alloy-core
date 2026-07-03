package project

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ProjectRoutes(r *gin.Engine,db *pgxpool.Pool) {
	api := r.Group("/project")

	service := NewService(db)
	handler := NewHandler(service)
	
	api.POST("/create",handler.CreateProject)
	api.GET("/get/:user_id",handler.GetUserProjects)
	api.DELETE("/:project_id",handler.DeleteProject)
}