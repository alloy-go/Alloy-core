package services

import (
	"context"
	"time"
	"github.com/jackc/pgx/v5"
	"github.com/minimon-cd/Alloy-core/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MetricsService aggregates metrics from all sources
type MetricsService struct {
	db                *pgxpool.Pool
	kubeService       *KubeMetricsService
	doraService       *DORAMetricsService
}

// NewMetricsService creates a new metrics aggregation service
func NewMetricsService(
	db *pgxpool.Pool,
	kubeService *KubeMetricsService,
	doraService *DORAMetricsService,
) *MetricsService {
	return &MetricsService{
		db:          db,
		kubeService: kubeService,
		doraService: doraService,
	}
}


// GetProjectMetrics fetches and aggregates all metrics for a project
func (s *MetricsService) GetProjectMetrics(
	ctx context.Context,
	projectID string,
) (*models.MetricsResponse, error) {

	projectInfo, err := s.getProjectInfo(ctx, projectID)
	if err != nil {
		return nil, err
	}

	req := models.MetricsRequest{
		ProjectID:      projectID,
		ProjectName:    projectInfo.ProjectName,
		DeploymentName: projectInfo.DeploymentName,
		Namespace:      projectInfo.Namespace,
		KubeConfig:     projectInfo.KubeConfig,
	}

	kubeMetrics, err := s.kubeService.GetKubernetesMetrics(ctx, req)
	if err != nil {
		return nil, err
	}

	doraMetrics, err := s.doraService.GetDORAMetrics(ctx, projectID)
	if err != nil {
		return nil, err
	}

	health := s.calculateHealth(kubeMetrics)

	_ = s.saveMetricsSnapshot(
		ctx,
		projectID,
		projectInfo.LatestDeploymentID,
		kubeMetrics,
		health,
	)

	return &models.MetricsResponse{
		ProjectID:   projectID,
		ProjectName: projectInfo.ProjectName,
		Timestamp:   time.Now(),
		Deployment: models.DeploymentMetrics{
			LatestID:   projectInfo.LatestDeploymentID,
			Status:     projectInfo.LatestStatus,
			ImageTag:   projectInfo.LatestImageTag,
			CommitSHA:  projectInfo.LatestCommitSHA,
			DeployedAt: projectInfo.DeployedAt,
		},
		Kubernetes: *kubeMetrics,
		DORA:       *doraMetrics,
		Health:     health,
	}, nil
}


// projectInfoResult holds project and deployment info from DB
type projectInfoResult struct {
	ProjectName          string
	DeploymentName       string
	Namespace            string
	LatestDeploymentID   string
	LatestStatus         string
	LatestImageTag       string
	LatestCommitSHA      string
	DeployedAt           time.Time
	KubeConfig           models.KubeConfig
}

// getProjectInfo retrieves project details and latest deployment from DB
func (s *MetricsService) getProjectInfo(ctx context.Context, projectID string) (*projectInfoResult, error) {
	var result projectInfoResult

	err := s.db.QueryRow(ctx, `
		SELECT 
			p.project_name,
			p.context_name,
			u.config_path,
			COALESCE(d.deployment_name, ''),
			COALESCE(d.namespace, 'default'),
			COALESCE(d.deployment_id::text, ''),
			COALESCE(d.status, ''),
			COALESCE(d.image_tag, ''),
			COALESCE(d.commit_sha, ''),
			COALESCE(d.created_at, NOW())
		FROM projects p
		JOIN users u ON p.user_id = u.user_id
		LEFT JOIN LATERAL (
			SELECT * FROM deployments
			WHERE project_id = p.project_id
			ORDER BY created_at DESC
			LIMIT 1
		) d ON true
		WHERE p.project_id = $1
	`, projectID).Scan(
		&result.ProjectName,
		&result.KubeConfig.ContextName,
		&result.KubeConfig.KubeconfigPath,
		&result.DeploymentName,
		&result.Namespace,
		&result.LatestDeploymentID,
		&result.LatestStatus,
		&result.LatestImageTag,
		&result.LatestCommitSHA,
		&result.DeployedAt,
	)

	if err != nil {
		return nil, err
	}

	result.KubeConfig.Namespace = result.Namespace
	return &result, nil
}

// calculateHealth computes health score and status
func (s *MetricsService) calculateHealth(
	k8s *models.KubernetesMetrics,
) models.HealthMetrics {

	score := 100

	// Pod readiness
	if k8s.Pods.Total > 0 {
		readyPercent :=
			float64(k8s.Pods.Ready) /
			float64(k8s.Pods.Total) * 100

		if readyPercent < 50 {
			score -= 40
		} else if readyPercent < 80 {
			score -= 20
		}
	}

	// Restarts
	if k8s.Pods.Restarts > 10 {
		score -= 20
	} else if k8s.Pods.Restarts > 5 {
		score -= 10
	}

	// Deployment state
	switch k8s.Deployment.Status {
	case "failed":
		score -= 40
	case "degraded":
		score -= 20
	}

	if score < 0 {
		score = 0
	}

	status := "healthy"
	if score < 50 {
		status = "critical"
	} else if score < 80 {
		status = "degraded"
	}

	return models.HealthMetrics{
		Score:  score,
		Status: status,
	}
}

// saveMetricsSnapshot saves the current metrics snapshot to project_metrics table
func (s *MetricsService) saveMetricsSnapshot(
	ctx context.Context,
	projectID string,
	deploymentID string,
	k8s *models.KubernetesMetrics,
	health models.HealthMetrics,
) error {

	_, err := s.db.Exec(ctx, `
		INSERT INTO project_metrics (
			project_id,
			latest_deployment_id,
			total_pods,
			ready_pods,
			restart_count,
			replicas_desired,
			replicas_available,
			health_score,
			health_status,
			last_updated
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, NOW()
		)
		ON CONFLICT (project_id) DO UPDATE SET
			latest_deployment_id = EXCLUDED.latest_deployment_id,
			total_pods = EXCLUDED.total_pods,
			ready_pods = EXCLUDED.ready_pods,
			restart_count = EXCLUDED.restart_count,
			replicas_desired = EXCLUDED.replicas_desired,
			replicas_available = EXCLUDED.replicas_available,
			health_score = EXCLUDED.health_score,
			health_status = EXCLUDED.health_status,
			last_updated = NOW()
	`,
		projectID,
		nullUUID(deploymentID),
		k8s.Pods.Total,
		k8s.Pods.Ready,
		k8s.Pods.Restarts,
		k8s.Deployment.ReplicasDesired,
		k8s.Deployment.ReplicasAvailable,
		health.Score,
		health.Status,
	)

	return err
}

func (s *MetricsService) GetProjectMetricsSummary(
	ctx context.Context,
	projectID string,
) (*models.ProjectMetricsSummary, error) {

	var summary models.ProjectMetricsSummary

	err := s.db.QueryRow(ctx, `
		SELECT
			project_id,
			COALESCE(total_pods, 0),
			COALESCE(ready_pods, 0),
			COALESCE(restart_count, 0),
			COALESCE(replicas_desired, 0),
			COALESCE(replicas_available, 0),
			COALESCE(health_score, 100),
			COALESCE(health_status, 'unknown'),
			last_updated
		FROM project_metrics
		WHERE project_id = $1
	`, projectID).Scan(
		&summary.ProjectID,
		&summary.Pods.Total,
		&summary.Pods.Ready,
		&summary.Pods.Restarts,
		&summary.Deployment.DesiredReplicas,
		&summary.Deployment.AvailableReplicas,
		&summary.Health.Score,
		&summary.Health.Status,
		&summary.LastUpdated,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return &models.ProjectMetricsSummary{
				ProjectID: projectID,
				Health: models.HealthMetrics{
					Score:  100,
					Status: "unknown",
				},
			}, nil
		}
		return nil, err
	}

	return &summary, nil
}


func nullUUID(id string) *string {
	if id == "" {
		return nil
	}
	return &id
}
