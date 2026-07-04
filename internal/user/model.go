package user

import (
	"time"
)

type UserProfile struct {
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	ConfigPath string    `json:"config_path"`
	CreatedAt  time.Time `json:"created_at"`
}