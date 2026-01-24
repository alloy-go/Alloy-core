package main

import (
	"log"
	"os"
	"time"

	services "github.com/minimon-cd/Alloy-core/client-go"
	"github.com/minimon-cd/Alloy-core/config"
	"github.com/minimon-cd/Alloy-core/routes"
	"github.com/gin-contrib/cors"
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
	origin := os.Getenv("ORIGIN")
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{origin},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	routes.RegisterRoutes(r, db)

	services.NewDeploymentWatcher(db).Start()

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"Message": "Welcome to Alloy-core",
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
