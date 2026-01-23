package utils

import (
	"context"
	 "time"
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
// GET USERS data
// ------------------------
type UserProfile struct {
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	ConfigPath string    `json:"config_path"`
	CreatedAt  time.Time `json:"created_at"`
}

func (us *UserService) GetUserByID(
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
