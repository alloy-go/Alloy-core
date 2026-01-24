package services

import (
	"context"
	"time"

	"github.com/minimon-cd/Alloy-core/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DORAMetricsService calculates DORA metrics from deployment history
type DORAMetricsService struct {
	db *pgxpool.Pool
}

// NewDORAMetricsService creates a new DORA metrics service
func NewDORAMetricsService(db *pgxpool.Pool) *DORAMetricsService {
	return &DORAMetricsService{db: db}
}

// GetDORAMetrics calculates DORA metrics for the last 30 days
func (s *DORAMetricsService) GetDORAMetrics(ctx context.Context, projectID string) (*models.DORAMetrics, error) {
	now := time.Now()
	periodStart := now.AddDate(0, 0, -30) // Last 30 days
	period := models.TimePeriod{
		Start: periodStart,
		End:   now,
	}

	// 1. Deployment Frequency
	deploymentFreq, err := s.getDeploymentFrequency(ctx, projectID, periodStart, now)
	if err != nil {
		return nil, err
	}

	// 2. Lead Time for Changes
	leadTime, err := s.getLeadTime(ctx, projectID, periodStart, now)
	if err != nil {
		return nil, err
	}

	// 3. Mean Time to Recovery
	mttr, err := s.getMTTR(ctx, projectID, periodStart, now)
	if err != nil {
		return nil, err
	}

	// 4. Change Failure Rate
	changeFailureRate, err := s.getChangeFailureRate(ctx, projectID, periodStart, now)
	if err != nil {
		return nil, err
	}

	// 5. Rollback Rate
	rollbackRate, err := s.getRollbackRate(ctx, projectID, periodStart, now)
	if err != nil {
		return nil, err
	}

	return &models.DORAMetrics{
		DeploymentFrequency: *deploymentFreq,
		LeadTime:            *leadTime,
		MTTR:                *mttr,
		ChangeFailureRate:   changeFailureRate,
		RollbackRate:        rollbackRate,
		Period:              period,
	}, nil
}

// getDeploymentFrequency calculates deployments per day
func (s *DORAMetricsService) getDeploymentFrequency(ctx context.Context, projectID string, start, end time.Time) (*models.DeploymentFrequencyMetrics, error) {
	var count int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM deployments 
		WHERE project_id = $1 
		  AND status = 'ready'
		  AND created_at BETWEEN $2 AND $3
	`, projectID, start, end).Scan(&count)

	if err != nil {
		return nil, err
	}

	days := end.Sub(start).Hours() / 24
	perDay := float64(count) / days

	return &models.DeploymentFrequencyMetrics{
		PerDay:       perDay,
		TotalLast30d: count,
	}, nil
}

// getLeadTime calculates average and median lead time (commit to production)
// Assumes created_at is when commit was made, updated_at is when deployed
func (s *DORAMetricsService) getLeadTime(ctx context.Context, projectID string, start, end time.Time) (*models.LeadTimeMetrics, error) {
	var avgMinutesF, medianMinutesF float64

	// Average lead time
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (updated_at - created_at)) / 60), 0)
		FROM deployments
		WHERE project_id = $1
		  AND status = 'ready'
		  AND created_at BETWEEN $2 AND $3
	`, projectID, start, end).Scan(&avgMinutesF)

	if err != nil {
		return nil, err
	}

	// Median lead time (using PERCENTILE_CONT)
	err = s.db.QueryRow(ctx, `
		SELECT COALESCE(
			PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (updated_at - created_at)) / 60),
			0
		)
		FROM deployments
		WHERE project_id = $1
		  AND status = 'ready'
		  AND created_at BETWEEN $2 AND $3
	`, projectID, start, end).Scan(&medianMinutesF)

	if err != nil {
		return nil, err
	}

	return &models.LeadTimeMetrics{
		AvgMinutes:    int(avgMinutesF),
		MedianMinutes: int(medianMinutesF),
	}, nil
}

// getMTTR calculates mean time to recovery
func (s *DORAMetricsService) getMTTR(ctx context.Context, projectID string, start, end time.Time) (*models.MTTRMetrics, error) {
	var incidentCount int
	var avgMinutesF float64
	// Count incidents (failed deployments)
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM deployments
		WHERE project_id = $1
		  AND status = 'failed'
		  AND created_at BETWEEN $2 AND $3
	`, projectID, start, end).Scan(&incidentCount)

	if err != nil {
		return nil, err
	}

	// Calculate average recovery time
	// Find time between failure and next successful deployment
	err = s.db.QueryRow(ctx, `
		WITH failures AS (
			SELECT deployment_id, created_at as failed_at
			FROM deployments
			WHERE project_id = $1
			  AND status = 'failed'
			  AND created_at BETWEEN $2 AND $3
		),
		recoveries AS (
			SELECT 
				f.deployment_id,
				f.failed_at,
				MIN(d.created_at) as recovered_at
			FROM failures f
			JOIN deployments d ON d.project_id = $1
			WHERE d.status = 'ready'
			  AND d.created_at > f.failed_at
			GROUP BY f.deployment_id, f.failed_at
		)
		SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (recovered_at - failed_at)) / 60), 0)
		FROM recoveries
	`, projectID, start, end).Scan(&avgMinutesF)

	if err != nil {
		avgMinutesF = 0.0
	}

	return &models.MTTRMetrics{
		AvgMinutes:       int(avgMinutesF),
		IncidentsLast30d: incidentCount,
	}, nil
}

// getChangeFailureRate calculates percentage of deployments that failed
func (s *DORAMetricsService) getChangeFailureRate(ctx context.Context, projectID string, start, end time.Time) (float64, error) {
	var total, failed int

	err := s.db.QueryRow(ctx, `
		SELECT 
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'failed') as failed
		FROM deployments
		WHERE project_id = $1
		  AND created_at BETWEEN $2 AND $3
	`, projectID, start, end).Scan(&total, &failed)

	if err != nil || total == 0 {
		return 0, err
	}

	return (float64(failed) / float64(total)) * 100, nil
}

// getRollbackRate calculates percentage of deployments that were rolled back
func (s *DORAMetricsService) getRollbackRate(ctx context.Context, projectID string, start, end time.Time) (float64, error) {
	var total, rollbacks int

	err := s.db.QueryRow(ctx, `
		SELECT 
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE rollback_from IS NOT NULL) as rollbacks
		FROM deployments
		WHERE project_id = $1
		  AND created_at BETWEEN $2 AND $3
	`, projectID, start, end).Scan(&total, &rollbacks)

	if err != nil || total == 0 {
		return 0, err
	}

	return (float64(rollbacks) / float64(total)) * 100, nil
}