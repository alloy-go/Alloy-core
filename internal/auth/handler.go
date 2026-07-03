package auth

import (
	"errors"
	"net/http"
	"strings"
	
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	Service *Service
}

func NewHandler(auth *Service) *Handler{
	return &Handler{
		Service: auth,
	}
}

func (ah *Handler) Signup(c *gin.Context){
	ctx := c.Request.Context()

	var req struct {
		Username   string `json:"username"`
		Email      string `json:"email"`
		Password   string `json:"password"`
		Token      string `json:"token"`
		ConfigPath string `json:"config_path"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := ah.Service.Signup(
		ctx,
		req.Username,
		req.Email,
		req.Password,
		req.Token,
		req.ConfigPath,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// 23505 is unique constraint violation
			if pgErr.Code == "23505" {
				if strings.Contains(pgErr.ConstraintName, "email") ||
					strings.Contains(pgErr.Detail, "email") {
					c.JSON(http.StatusConflict, gin.H{
						"error": "Email already exists",
					})
					return
				}
				if strings.Contains(pgErr.ConstraintName, "username") ||
					strings.Contains(pgErr.Detail, "username") {
					c.JSON(http.StatusConflict, gin.H{
						"error": "Username already exists",
					})
					return
				}
				c.JSON(http.StatusConflict, gin.H{
					"error": "Record already exists",
				})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Signup failed",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user_id": userID,
	})
}

func (ah *Handler) Login(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := ah.Service.Login(
		ctx,
		req.Username,
		req.Password,
	)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid credentials",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
	})
}
