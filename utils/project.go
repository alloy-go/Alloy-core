package utils

import (
	"context"
	"fmt"

	"github.com/minimon-cd/Alloy-core/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectService struct {
	DB *pgxpool.Pool
}

func NewProjectService(db *pgxpool.Pool) *ProjectService {
	return &ProjectService{DB: db}
}

func (p *ProjectService) CreateProject(
	ctx context.Context,
	userID, name, deployType, contextName string,
) error {

	_, err := p.DB.Exec(ctx, `
		INSERT INTO projects (user_id, project_name, deployment_type, context_name)
		VALUES ($1,$2,$3,$4)
	`,
		userID, name, deployType, contextName)

	return err
}

// Not using Anywhere
func (p *ProjectService) GetProjectsByUser(ctx context.Context, userID string) ([]models.Project, error) {
	rows, err := p.DB.Query(ctx, `
        SELECT project_id, user_id, project_name, deployment_type, context_name, created_at
        FROM projects
        WHERE user_id = $1
        ORDER BY created_at DESC
    `, userID)
	if err != nil {
		fmt.Println("DB query error:", err)
		return nil, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var pr models.Project
		if err := rows.Scan(&pr.ProjectID, &pr.UserID, &pr.ProjectName, &pr.DeploymentType, &pr.ContextName, &pr.CreatedAt); err != nil {
			fmt.Println("DB scan error:", err)
			return nil, err
		}
		projects = append(projects, pr)
	}

	return projects, nil
}

func (ps *ProjectService) GetProjectsWithDeploymentInfo(ctx context.Context, userID string) ([]models.ProjectWithDeploymentInfo, error) {
	query := `
        WITH latest_deployments AS (
            SELECT DISTINCT ON (d.project_id)
                d.project_id,
                d.status,
                d.commit_sha,
                d.image_tag,
                d.namespace,
                d.deployment_name,
                d.updated_at,
                d.error_message,
                d.deployment_type
            FROM deployments d
            ORDER BY d.project_id, d.updated_at DESC
        ),
        deployment_stats AS (
            SELECT 
                d.project_id,
                COUNT(*) as total_deployments,
                SUM(CASE WHEN d.status = 'ready' THEN 1 ELSE 0 END) as ready_count,
                SUM(CASE WHEN d.status = 'failed' THEN 1 ELSE 0 END) as failed_count,
                SUM(CASE WHEN d.status NOT IN ('ready', 'failed') THEN 1 ELSE 0 END) as processing_count,
                BOOL_OR(d.deployment_type = 'canary') as canary_active
            FROM deployments d
            GROUP BY d.project_id
        )
        SELECT 
            p.project_name,
            p.deployment_type,
            p.created_at,
            
            ld.status as latest_status,
            ld.deployment_name as latest_deployment_name,
            ld.updated_at as latest_updated_at,
            ld.error_message as latest_error_message,
            ld.deployment_type as latest_deployment_type,
            
            COALESCE(ds.total_deployments, 0) as total_deployments,
            COALESCE(ds.ready_count, 0) as ready_count,
            COALESCE(ds.failed_count, 0) as failed_count,
            COALESCE(ds.processing_count, 0) as processing_count,
            COALESCE(ds.canary_active, false) as canary_active
        FROM projects p
        LEFT JOIN latest_deployments ld ON p.project_id = ld.project_id
        LEFT JOIN deployment_stats ds ON p.project_id = ds.project_id
        WHERE p.user_id = $1
        ORDER BY p.created_at DESC
    `

	rows, err := ps.DB.Query(ctx, query, userID)
	if err != nil {
		fmt.Println("GetProjectsWithDeploymentInfo - DB query error:", err)
		return nil, err
	}
	defer rows.Close()

	var projects []models.ProjectWithDeploymentInfo
	for rows.Next() {
		var p models.ProjectWithDeploymentInfo
		err := rows.Scan(
			&p.ProjectName,
			&p.ContextName,
			&p.CreatedAt,

			&p.LatestStatus,
			&p.LatestDeploymentName,
			&p.LatestUpdatedAt,
			&p.LatestErrorMessage,
			&p.LatestDeploymentType,

			&p.TotalDeployments,
			&p.ReadyCount,
			&p.FailedCount,
			&p.ProcessingCount,
			&p.CanaryActive,
		)
		if err != nil {
			fmt.Println("GetProjectsWithDeploymentInfo - scan error:", err)
			return nil, err
		}
		projects = append(projects, p)
	}

	if err = rows.Err(); err != nil {
		fmt.Println("GetProjectsWithDeploymentInfo - rows error:", err)
		return nil, err
	}

	return projects, nil
}

//Delete project
func (ps *ProjectService) DeleteProject(
	ctx context.Context,
	projectID string,
	userID string,
) error {

	cmd, err := ps.DB.Exec(ctx, `
		DELETE FROM projects
		WHERE project_id = $1
		  AND user_id = $2
	`, projectID, userID)

	if err != nil {
		return err
	}

	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("project not found or not owned by user")
	}

	return nil
}
