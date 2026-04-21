package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nicholasricci/caddy-dashboard/internal/auth"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"go.uber.org/zap"
)

type AuthHandler struct {
	authSvc *auth.Service
	logger  *zap.Logger
}

func NewAuthHandler(authSvc *auth.Service, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, logger: nopLogger(logger)}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Login godoc
// @Summary User login
// @Description Authenticates user and returns access and refresh JWTs
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body loginRequest true "Login payload"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	pair, err := h.authSvc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid credentials"})
			return
		}
		logRequestError(h.logger, c, "login failed", err, zap.String("login_username", req.Username))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "login failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"token":         pair.AccessToken,
	})
}

// Refresh godoc
// @Summary Refresh token
// @Description Issues a new access and refresh token pair
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body refreshRequest true "Refresh payload"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	pair, err := h.authSvc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, auth.ErrRefreshInvalid) {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid refresh token"})
			return
		}
		logRequestError(h.logger, c, "refresh failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "refresh failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"token":         pair.AccessToken,
	})
}

// Logout godoc
// @Summary Logout user
// @Description Revokes the provided refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body logoutRequest true "Logout payload"
// @Success 204
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	if err := h.authSvc.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		logRequestError(h.logger, c, "logout failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "logout failed"})
		return
	}
	c.Status(http.StatusNoContent)
}
