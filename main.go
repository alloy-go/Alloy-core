package main

import (
	"log"
	"os"

	"github.com/Santhoshkumar044/MiniMon/config"
	"github.com/Santhoshkumar044/MiniMon/routes"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// Load DB config
	db := config.InitDB()
	defer db.Close()

	config.RunMigrations(db)
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	routes.RegisterRoutes(r, db)

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"Message": "Welcome to Minimon",
		})
	})

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
