package controllers

import (
	"encoding/base64"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
)

type WebhookPayload struct {
	ProjectID string `json:"project_id"`
	ImageTag  string `json:"image_tag"`
	CommitSHA string `json:"commit_sha"`
	Files     struct {
		Secret     string `json:"secret"`
		Service    string `json:"service"`
		Deployment string `json:"deployment"`
	} `json:"files"`
}

func DeployWebhook(c *gin.Context) {
	var payload WebhookPayload

	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Printf("❌ Invalid payload: %v\n", err)
		c.JSON(400, gin.H{"error": "Invalid payload"})
		return
	}

	// Decode the YAML files
	secretYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Secret)
	serviceYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Service)
	deploymentYAML, _ := base64.StdEncoding.DecodeString(payload.Files.Deployment)

	// Log everything
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("📡 NEW DEPLOYMENT RECEIVED")
	log.Printf("   Project: %s\n", payload.ProjectID)
	log.Printf("   Image: %s\n", payload.ImageTag)
	log.Printf("   Commit: %s\n", payload.CommitSHA)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	log.Println("\n📄 secret.yaml:")
	fmt.Println(string(secretYAML))

	log.Println("\n📄 service.yaml:")
	fmt.Println(string(serviceYAML))

	log.Println("\n📄 deployment.yaml:")
	fmt.Println(string(deploymentYAML))

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	c.JSON(200, gin.H{"status": "received"})
}
