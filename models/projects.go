package models

import "time"

type Project struct {
	ProjectID      string    `json:"project_id"`
	UserID         string    `json:"user_id"`
	ProjectName    string    `json:"project_name"`
	DeploymentType string    `json:"deployment_type"`
	ContextName    string    `json:"context_name"`
	CreatedAt      time.Time `json:"created_at"`
}
