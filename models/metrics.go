package models

import "time"

// ============================================================================
// API RESPONSE MODELS
// ============================================================================

// MetricsResponse is the complete metrics response for a project
type MetricsResponse struct {
	ProjectID   string              `json:"project_id"`
	ProjectName string              `json:"project_name"`
	Timestamp   time.Time           `json:"timestamp"`
	Deployment  DeploymentMetrics   `json:"deployment"`
	Kubernetes  KubernetesMetrics   `json:"kubernetes"`
	DORA        DORAMetrics         `json:"dora"`
	Health      HealthMetrics       `json:"health"`
}

// ============================================================================
// DEPLOYMENT METRICS
// ============================================================================

type DeploymentMetrics struct {
	LatestID   string    `json:"latest_id"`
	Status     string    `json:"status"`
	ImageTag   string    `json:"image_tag"`
	CommitSHA  string    `json:"commit_sha"`
	DeployedAt time.Time `json:"deployed_at"`
}

// ============================================================================
// KUBERNETES METRICS
// ============================================================================

type KubernetesMetrics struct {
	Pods       PodMetrics       `json:"pods"`
	Deployment DeploymentStatus `json:"deployment"`
	Resources  *ResourceMetrics `json:"resources,omitempty"`
}

type PodMetrics struct {
	Total    int32        `json:"total"`
	Ready    int32        `json:"ready"`
	Restarts int32        `json:"restarts"`
	Details  []PodDetails `json:"details,omitempty"`
}

type PodDetails struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Ready     bool      `json:"ready"`
	Restarts  int32     `json:"restarts"`
	CreatedAt time.Time `json:"created_at"`
	NodeName  string    `json:"node_name,omitempty"`
}

type DeploymentStatus struct {
	ReplicasDesired   int32  `json:"replicas_desired"`
	ReplicasAvailable int32  `json:"replicas_available"`
	ReplicasReady     int32  `json:"replicas_ready"`
	Status            string `json:"status"` // healthy, progressing, failed
}

type ResourceMetrics struct {
	CPUUsageCores float64 `json:"cpu_usage_cores"`
	MemoryUsageMB float64 `json:"memory_usage_mb"`
}

type TimePeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ============================================================================
// DORA METRICS
// ============================================================================

type DORAMetrics struct {
	DeploymentFrequency DeploymentFrequencyMetrics `json:"deployment_frequency"`
	LeadTime            LeadTimeMetrics            `json:"lead_time"`
	MTTR                MTTRMetrics                `json:"mttr"`
	ChangeFailureRate   float64                    `json:"change_failure_rate"` // percentage
	RollbackRate        float64                    `json:"rollback_rate"`       // percentage
	Period              TimePeriod                 `json:"period"`
}

type DeploymentFrequencyMetrics struct {
	PerDay       float64 `json:"per_day"`
	TotalLast30d int     `json:"total_last_30d"`
}

type LeadTimeMetrics struct {
	AvgMinutes    int `json:"avg_minutes"`
	MedianMinutes int `json:"median_minutes"`
}

type MTTRMetrics struct {
	AvgMinutes       int `json:"avg_minutes"`
	IncidentsLast30d int `json:"incidents_last_30d"`
}

// ============================================================================
// HEALTH METRICS
// ============================================================================

type HealthMetrics struct {
	Score  int    `json:"score"`  // 0-100
	Status string `json:"status"` // healthy, degraded, critical
}

// ============================================================================
// DATABASE MODELS
// ============================================================================

// ProjectMetric represents the project_metrics table
type ProjectMetric struct {
	MetricID           string
	ProjectID          string
	LatestDeploymentID *string
	LatestImageTag     *string
	LatestCommitSHA    *string
	LatestStatus       *string
	DeployedAt         *time.Time
	
	// Kubernetes
	TotalPods         int32
	ReadyPods         int32
	RestartCount      int32
	ReplicasDesired   int32
	ReplicasAvailable int32
	CPUUsageCores     *float64
	MemoryUsageMB     *float64
	
	// Application
	RequestRate    *float64
	ErrorRate      *float64
	AvgLatencyMS   *float64
	P95LatencyMS   *float64
	P99LatencyMS   *float64
	Status2xxCount *int64
	Status4xxCount *int64
	Status5xxCount *int64
	
	// Health
	HealthScore  int
	HealthStatus string
	
	LastUpdated time.Time
}

// DORAMetric represents the dora_metrics table
type DORAMetric struct {
	MetricID              string
	ProjectID             string
	DeploymentCount       int
	DeploymentsPerDay     float64
	AvgLeadTimeMinutes    *int
	MedianLeadTimeMinutes *int
	AvgMTTRMinutes        *int
	IncidentCount         int
	TotalDeployments      int
	FailedDeployments     int
	ChangeFailureRate     float64
	RollbackCount         int
	RollbackRate          float64
	PeriodStart           time.Time
	PeriodEnd             time.Time
	CalculatedAt          time.Time
}

// ============================================================================
// INTERNAL SERVICE TYPES
// ============================================================================

// KubeConfig holds Kubernetes connection info
type KubeConfig struct {
	KubeconfigPath string
	ContextName    string
	Namespace      string
}

// MetricsRequest is used internally for service communication
type MetricsRequest struct {
	ProjectID      string
	ProjectName    string
	DeploymentName string
	Namespace      string
	KubeConfig     KubeConfig
}

//Summary metrics

type ProjectMetricsSummary struct {
	ProjectID string    `json:"project_id"`

	Pods struct {
		Total    int32 `json:"total"`
		Ready    int32 `json:"ready"`
		Restarts int32 `json:"restarts"`
	} `json:"pods"`

	Deployment struct {
		DesiredReplicas   int32 `json:"desired_replicas"`
		AvailableReplicas int32 `json:"available_replicas"`
	} `json:"deployment"`

	Health struct {
		Score  int    `json:"score"`
		Status string `json:"status"`
	} `json:"health"`

	LastUpdated time.Time `json:"last_updated"`
}
