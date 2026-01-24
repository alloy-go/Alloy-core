package models

import "time"

type ProjectWithDeploymentInfo struct {
	ProjectId            string     `json:"project_id"`
	ProjectName          string     `json:"project_name"`
	ContextName          string     `json:"context_name"`
	LatestStatus         *string    `json:"latest_status,omitempty"`
	LatestDeploymentName *string    `json:"latest_deployment_name,omitempty"`
	LatestUpdatedAt      *time.Time `json:"latest_updated_at,omitempty"`
	LatestErrorMessage   *string    `json:"latest_error_message,omitempty"`
	LatestDeploymentType *string    `json:"latest_deployment_type,omitempty"`
	TotalDeployments     int        `json:"total_deployments"`
	ReadyCount           int        `json:"ready_count"`
	FailedCount          int        `json:"failed_count"`
	ProcessingCount      int        `json:"processing_count"`
	CanaryActive         bool       `json:"canary_active"`
	CreatedAt            time.Time  `json:"created_at"`
}
