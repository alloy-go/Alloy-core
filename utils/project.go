package utils

import (
	"context"

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
