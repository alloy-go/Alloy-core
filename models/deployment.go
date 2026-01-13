package models

import "time"

type DeploymentRecord struct {
    DeploymentID     string    `json:"deployment_id"`
    ProjectID        string    `json:"project_id"`
    CommitSHA        string    `json:"commit_sha"`
    ImageTag         string    `json:"image_tag"`
    Status           string    `json:"status"`
    Namespace        string    `json:"namespace"`
    DeploymentName   string    `json:"deployment_name"`
    DeploymentType   string    `json:"deployment_type"`
    CanaryTrack      string    `json:"canary_track"`
    CanaryStage      *int      `json:"canary_stage"`
    CanaryTargetReplicas *int  `json:"canary_target_replicas"`
    RollbackFrom     string    `json:"rollback_from"`
    CreatedAt        time.Time `json:"created_at"`
    UpdatedAt        time.Time `json:"updated_at"`
    ErrorMessage     string    `json:"error_message"`
    FailureType      string    `json:"failure_type"`     // 'hard', 'soft', NULL
    NeedsRollback    bool      `json:"needs_rollback"`   // true/false
}
