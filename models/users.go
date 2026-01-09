package models

import "time"

type User struct {
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Token        string    `json:"token"`
	ConfigPath   string    `json:"config_path"`
	CreatedAt    time.Time `json:"created_at"`
}
