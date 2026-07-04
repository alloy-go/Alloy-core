package user

import (
	"context"
	
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	DB *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		DB : db,
	}
}

func (us *Service) UpdateKubeConfigPath(
	ctx context.Context,
	userID string,
	configPath string,
) error {

	_, err := us.DB.Exec(ctx, `
		UPDATE users
		SET config_path = $1
		WHERE user_id = $2
	`, configPath, userID)

	return err
}


func (us *Service) GetUserByID(
	ctx context.Context,
	userID string,
) (*UserProfile, error) {

	var u UserProfile

	err := us.DB.QueryRow(ctx, `
		SELECT 
			user_id,
			username,
			email,
			config_path,
			created_at
		FROM users
		WHERE user_id = $1
	`, userID).Scan(
		&u.UserID,
		&u.Username,
		&u.Email,
		&u.ConfigPath,
		&u.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &u, nil
}
