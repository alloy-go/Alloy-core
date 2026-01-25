package services

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minimon-cd/Alloy-core/models"
)

type CanaryMetricsService struct {
	db          *pgxpool.Pool
	kubeService *KubeMetricsService
}

func NewCanaryMetricsService(db *pgxpool.Pool, kubeService *KubeMetricsService) *CanaryMetricsService {
	return &CanaryMetricsService{
		db:          db,
		kubeService: kubeService,
	}
}

// CaptureCanarySnapshot takes a snapshot of canary metrics at a specific point in time
func (s *CanaryMetricsService) CaptureCanarySnapshot(
	ctx context.Context,
	deploymentID string,
	projectID string,
	canaryDeploymentName string,
	namespace string,
	stage int,
	kubeConfig models.KubeConfig,
) error {
	
	log.Printf("📸 Capturing canary snapshot for deployment %s (stage %d)", deploymentID, stage)
	
	// Fetch CURRENT live metrics from Kubernetes for the canary deployment
	req := models.MetricsRequest{
		ProjectID:      projectID,
		DeploymentName: canaryDeploymentName,
		Namespace:      namespace,
		KubeConfig:     kubeConfig,
	}
	
	k8sMetrics, err := s.kubeService.GetKubernetesMetrics(ctx, req)
	if err != nil {
		log.Printf("⚠️  Warning: failed to fetch canary k8s metrics: %v", err)
		// Don't fail - store what we can
		k8sMetrics = &models.KubernetesMetrics{
			Pods: models.PodMetrics{
				Total:    0,
				Ready:    0,
				Restarts: 0,
			},
			Deployment: models.DeploymentStatus{
				Status: "unknown",
			},
		}
	}
	
	// Calculate health for this snapshot
	health := calculateCanaryHealth(k8sMetrics)
	
	// Get analysis window from deployment record
	var analysisWindow int
	err = s.db.QueryRow(ctx, `
		SELECT COALESCE(canary_analysis_window, 5)
		FROM deployments
		WHERE deployment_id = $1
	`, deploymentID).Scan(&analysisWindow)
	
	if err != nil {
		analysisWindow = 5 // default fallback
	}
	
	// Store snapshot in database
	_, err = s.db.Exec(ctx, `
		INSERT INTO canary_metrics (
			deployment_id,
			project_id,
			canary_deployment_name,
			canary_namespace,
			canary_stage,
			snapshot_time,
			analysis_window_minutes,
			total_pods,
			ready_pods,
			restart_count,
			target_replicas,
			health_score,
			health_status,
			canary_result,
			promotion_decision,
			started_at
		) VALUES (
			$1, $2, $3, $4, $5, NOW(), $6,
			$7, $8, $9, $10, $11, $12,
			'analyzing', 'pending', NOW()
		)
		ON CONFLICT (deployment_id) DO UPDATE SET
			canary_stage = EXCLUDED.canary_stage,
			snapshot_time = NOW(),
			total_pods = EXCLUDED.total_pods,
			ready_pods = EXCLUDED.ready_pods,
			restart_count = EXCLUDED.restart_count,
			target_replicas = EXCLUDED.target_replicas,
			health_score = EXCLUDED.health_score,
			health_status = EXCLUDED.health_status
	`,
		deploymentID,
		projectID,
		canaryDeploymentName,
		namespace,
		stage,
		analysisWindow,
		k8sMetrics.Pods.Total,
		k8sMetrics.Pods.Ready,
		k8sMetrics.Pods.Restarts,
		k8sMetrics.Deployment.ReplicasDesired,
		health.Score,
		health.Status,
	)
	
	if err != nil {
		return fmt.Errorf("failed to save canary snapshot: %w", err)
	}
	
	log.Printf("✅ Canary snapshot saved: %d pods, %d ready, health=%d", 
		k8sMetrics.Pods.Total, k8sMetrics.Pods.Ready, health.Score)
	
	return nil
}

// FinalizeCanaryResult marks the canary as completed with a final result
func (s *CanaryMetricsService) FinalizeCanaryResult(
	ctx context.Context,
	deploymentID string,
	result string, // 'passed', 'failed', 'aborted'
	decision string, // 'promote', 'abort'
) error {
	
	log.Printf("🏁 Finalizing canary result: %s -> %s", result, decision)
	
	_, err := s.db.Exec(ctx, `
		UPDATE canary_metrics
		SET 
			canary_result = $2,
			promotion_decision = $3,
			completed_at = NOW()
		WHERE deployment_id = $1
	`, deploymentID, result, decision)
	
	if err != nil {
		return fmt.Errorf("failed to finalize canary result: %w", err)
	}
	
	return nil
}

// GetLatestCanaryMetrics retrieves the most recent canary snapshot for a project
func (s *CanaryMetricsService) GetLatestCanaryMetrics(
	ctx context.Context,
	projectID string,
) (*models.CanaryMetricsSnapshot, error) {
	
	var snapshot models.CanaryMetricsSnapshot
	
	err := s.db.QueryRow(ctx, `
		SELECT 
			canary_metric_id,
			deployment_id,
			project_id,
			canary_deployment_name,
			canary_namespace,
			canary_stage,
			snapshot_time,
			COALESCE(analysis_window_minutes, 0),
			COALESCE(total_pods, 0),
			COALESCE(ready_pods, 0),
			COALESCE(restart_count, 0),
			COALESCE(target_replicas, 0),
			COALESCE(health_score, 100),
			COALESCE(health_status, 'unknown'),
			COALESCE(error_rate, 0.0),
			COALESCE(canary_result, 'analyzing'),
			COALESCE(promotion_decision, 'pending'),
			started_at,
			completed_at
		FROM canary_metrics
		WHERE project_id = $1
		ORDER BY snapshot_time DESC
		LIMIT 1
	`, projectID).Scan(
		&snapshot.CanaryMetricID,
		&snapshot.DeploymentID,
		&snapshot.ProjectID,
		&snapshot.CanaryDeploymentName,
		&snapshot.CanaryNamespace,
		&snapshot.CanaryStage,
		&snapshot.SnapshotTime,
		&snapshot.AnalysisWindowMin,
		&snapshot.TotalPods,
		&snapshot.ReadyPods,
		&snapshot.RestartCount,
		&snapshot.TargetReplicas,
		&snapshot.HealthScore,
		&snapshot.HealthStatus,
		&snapshot.ErrorRate,
		&snapshot.CanaryResult,
		&snapshot.PromotionDecision,
		&snapshot.StartedAt,
		&snapshot.CompletedAt,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No canary exists
		}
		return nil, err
	}
	
	return &snapshot, nil
}

// GetCanaryByDeploymentID retrieves canary metrics for a specific deployment
func (s *CanaryMetricsService) GetCanaryByDeploymentID(
	ctx context.Context,
	deploymentID string,
) (*models.CanaryMetricsSnapshot, error) {
	
	var snapshot models.CanaryMetricsSnapshot
	
	err := s.db.QueryRow(ctx, `
		SELECT 
			canary_metric_id,
			deployment_id,
			project_id,
			canary_deployment_name,
			canary_namespace,
			canary_stage,
			snapshot_time,
			COALESCE(analysis_window_minutes, 0),
			COALESCE(total_pods, 0),
			COALESCE(ready_pods, 0),
			COALESCE(restart_count, 0),
			COALESCE(target_replicas, 0),
			COALESCE(health_score, 100),
			COALESCE(health_status, 'unknown'),
			COALESCE(error_rate, 0.0),
			COALESCE(canary_result, 'analyzing'),
			COALESCE(promotion_decision, 'pending'),
			started_at,
			completed_at
		FROM canary_metrics
		WHERE deployment_id = $1
	`, deploymentID).Scan(
		&snapshot.CanaryMetricID,
		&snapshot.DeploymentID,
		&snapshot.ProjectID,
		&snapshot.CanaryDeploymentName,
		&snapshot.CanaryNamespace,
		&snapshot.CanaryStage,
		&snapshot.SnapshotTime,
		&snapshot.AnalysisWindowMin,
		&snapshot.TotalPods,
		&snapshot.ReadyPods,
		&snapshot.RestartCount,
		&snapshot.TargetReplicas,
		&snapshot.HealthScore,
		&snapshot.HealthStatus,
		&snapshot.ErrorRate,
		&snapshot.CanaryResult,
		&snapshot.PromotionDecision,
		&snapshot.StartedAt,
		&snapshot.CompletedAt,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	
	return &snapshot, nil
}

func calculateCanaryHealth(k8s *models.KubernetesMetrics) models.HealthMetrics {
	score := 100
	
	// Pod readiness
	if k8s.Pods.Total > 0 {
		readyPercent := float64(k8s.Pods.Ready) / float64(k8s.Pods.Total) * 100
		if readyPercent < 50 {
			score -= 50
		} else if readyPercent < 80 {
			score -= 25
		}
	} else {
		// No pods is critical
		score = 0
	}
	
	// Restarts (canaries should have very few restarts)
	if k8s.Pods.Restarts > 5 {
		score -= 30
	} else if k8s.Pods.Restarts > 2 {
		score -= 15
	}
	
	// Deployment status
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
	} else if score < 75 {
		status = "degraded"
	}
	
	return models.HealthMetrics{
		Score:  score,
		Status: status,
	}
}