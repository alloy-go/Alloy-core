package utils

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	DB *pgxpool.Pool
}

func NewAuthService(db *pgxpool.Pool) *AuthService {
	return &AuthService{DB: db}
}

func (a *AuthService) Signup(
	ctx context.Context,
	username, email, password, token, configPath string,
) (string, error) {

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), 12)

	var userID string
	err := a.DB.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, token, config_path)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING user_id
	`,
		username, email, string(hash), token, configPath,
	).Scan(&userID)

	return userID, err
}

func (a *AuthService) Login(
	ctx context.Context,
	username, password string,
) (string, error) {

	var userID, hash string

	err := a.DB.QueryRow(ctx,
		`SELECT user_id, password_hash FROM users WHERE username=$1`,
		username,
	).Scan(&userID, &hash)

	if err != nil {
		return "", errors.New("user not found")
	}

	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", errors.New("invalid password")
	}

	return userID, nil
}
