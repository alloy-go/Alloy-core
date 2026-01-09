package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/Santhoshkumar044/MiniMon/config"
)

func main() {
	// Load DB config
	db := config.InitDB()
	defer db.Close()

	config.RunMigrations(db);
	gin.SetMode(gin.ReleaseMode);
	
	r := gin.Default()

	// Health route
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "Server is up",
			"db":     "connected",
		})
	})

	port := os.Getenv("PORT")

	log.Println("Server running on port", port)
	r.Run(":" + port)
}
