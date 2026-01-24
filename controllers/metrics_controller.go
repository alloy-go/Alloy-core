package controllers

import (
	"log"
	"net/http"
	 services "github.com/minimon-cd/Alloy-core/client-go"
	"github.com/gin-gonic/gin"
)

// MetricsController handles metrics-related HTTP requests
type MetricsController struct {
	metricsService *services.MetricsService
}

// NewMetricsController creates a new metrics controller
func NewMetricsController(metricsService *services.MetricsService) *MetricsController {
	return &MetricsController{
		metricsService: metricsService,
	}
}

// GetProjectMetrics handles GET /api/projects/:project_id/metrics
func (mc *MetricsController) GetProjectMetrics(c *gin.Context) {
	projectID := c.Param("project_id")

	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "project_id is required",
		})
		return
	}

	log.Printf("📊 Fetching metrics for project: %s", projectID)

	// Fetch metrics from service
	metrics, err := mc.metricsService.GetProjectMetrics(c.Request.Context(), projectID)
	if err != nil {
		log.Printf("❌ Failed to get metrics for project %s: %v", projectID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch metrics",
			"details": err.Error(),
		})
		return
	}

	log.Printf("✅ Successfully fetched metrics for project %s", projectID)

	c.JSON(http.StatusOK, metrics)
}

// GetProjectMetricsSummary handles GET /api/projects/:project_id/metrics/summary
// Returns cached metrics from DB for fast dashboard loading
func (mc *MetricsController) GetProjectMetricsSummary(c *gin.Context) {
	projectID := c.Param("project_id")

	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "project_id is required",
		})
		return
	}

	summary, err := mc.metricsService.GetProjectMetricsSummary(
		c.Request.Context(),
		projectID,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "metrics not available yet",
		})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// RefreshProjectMetrics handles POST /api/projects/:project_id/metrics/refresh
// Forces a fresh metrics collection
func (mc *MetricsController) RefreshProjectMetrics(c *gin.Context) {
	projectID := c.Param("project_id")

	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "project_id is required",
		})
		return
	}

	log.Printf("🔄 Refreshing metrics for project: %s", projectID)

	// Same as GetProjectMetrics but explicitly indicates a refresh
	metrics, err := mc.metricsService.GetProjectMetrics(c.Request.Context(), projectID)
	if err != nil {
		log.Printf("❌ Failed to refresh metrics for project %s: %v", projectID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to refresh metrics",
			"details": err.Error(),
		})
		return
	}

	log.Printf("✅ Successfully refreshed metrics for project %s", projectID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Metrics refreshed successfully",
		"data":    metrics,
	})
}