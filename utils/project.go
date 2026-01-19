package utils

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Santhoshkumar044/MiniMon/models"
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

func (p *ProjectService) GetProjectsByUser(ctx context.Context, userID string) ([]models.Project, error) {
    rows, err := p.DB.Query(ctx, `
        SELECT project_id, user_id, project_name, deployment_type, context_name, created_at
        FROM projects
        WHERE user_id = $1
        ORDER BY created_at DESC
    `, userID)
    if err != nil {
        // Log the actual DB error
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
