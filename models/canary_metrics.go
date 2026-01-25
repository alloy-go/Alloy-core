package models

import "time"

// CanaryMetricsSnapshot represents a point-in-time snapshot of canary metrics
type CanaryMetricsSnapshot struct {
	CanaryMetricID       string     `json:"canary_metric_id"`
	DeploymentID         string     `json:"deployment_id"`
	ProjectID            string     `json:"project_id"`
	
	// Canary Identity
	CanaryDeploymentName string     `json:"canary_deployment_name"`
	CanaryNamespace      string     `json:"canary_namespace"`
	CanaryStage          int        `json:"canary_stage"`
	
	// Snapshot Metadata
	SnapshotTime         time.Time  `json:"snapshot_time"`
	AnalysisWindowMin    int        `json:"analysis_window_minutes"`
	
	// Pod Metrics
	TotalPods            int32      `json:"total_pods"`
	ReadyPods            int32      `json:"ready_pods"`
	RestartCount         int32      `json:"restart_count"`
	TargetReplicas       int32      `json:"target_replicas"`
	
	// Health
	HealthScore          int        `json:"health_score"`
	HealthStatus         string     `json:"health_status"`
	ErrorRate            float64    `json:"error_rate"`
	
	// Result
	CanaryResult         string     `json:"canary_result"`
	PromotionDecision    string     `json:"promotion_decision"`
	
	// Timestamps
	StartedAt            time.Time  `json:"started_at"`
	CompletedAt          *time.Time `json:"completed_at,omitempty"`
}

// MetricsResponseV2 - Updated response with separated production and canary
type MetricsResponseV2 struct {
	ProjectID   string                  `json:"project_id"`
	ProjectName string                  `json:"project_name"`
	Timestamp   time.Time               `json:"timestamp"`
	
	// Current Production Deployment (live)
	Production  ProductionMetrics       `json:"production"`
	
	// Historical Canary Metrics (frozen snapshots)
	Canary      *CanaryMetricsSnapshot  `json:"canary,omitempty"`
	
	// DORA Metrics
	DORA        DORAMetrics             `json:"dora"`
}

type ProductionMetrics struct {
	// Latest stable deployment info
	Deployment  DeploymentMetrics `json:"deployment"`
	
	// Live Kubernetes state
	Kubernetes  KubernetesMetrics `json:"kubernetes"`
	
	// Current health
	Health      HealthMetrics     `json:"health"`
}