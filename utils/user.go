package utils

import (
	"context"
	 "github.com/Santhoshkumar044/MiniMon/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserService struct {
	DB *pgxpool.Pool
}

func NewUserService(db *pgxpool.Pool) *UserService {
	return &UserService{
		DB: db,
	}
}

// ------------------------
// UPDATE kube config path
// ------------------------
func (us *UserService) UpdateKubeConfigPath(
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

// ------------------------
// GET ALL USERS
// ------------------------

func (us *UserService) GetUserByID(
	ctx context.Context,
	userID string,
) (*models.User, error) {

	var u models.User

	err := us.DB.QueryRow(ctx, `
		SELECT 
			user_id,
			username,
			email,
			password_hash,
			token,
			config_path,
			created_at
		FROM users
		WHERE user_id = $1
	`, userID).Scan(
		&u.UserID,
		&u.Username,
		&u.Email,
		&u.PasswordHash,
		&u.Token,
		&u.ConfigPath,
		&u.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &u, nil
}
