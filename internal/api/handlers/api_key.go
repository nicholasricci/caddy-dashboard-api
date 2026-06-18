package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
	"go.uber.org/zap"
)

type APIKeyHandler struct {
	svc    *services.APIKeyService
	audit  *services.AuditService
	logger *zap.Logger
}

func NewAPIKeyHandler(svc *services.APIKeyService, audit *services.AuditService, logger *zap.Logger) *APIKeyHandler {
	return &APIKeyHandler{svc: svc, audit: audit, logger: nopLogger(logger)}
}

type createAPIKeyRequest struct {
	Name                      string   `json:"name" binding:"required"`
	Scopes                    []string `json:"scopes" binding:"required" example:"register_upstream"`
	AllowedDiscoveryConfigIDs []string `json:"allowed_discovery_config_ids" binding:"required"`
	ExpiresAt                 *string  `json:"expires_at"`
}

// List godoc
// @Summary List API keys
// @Tags api-keys
// @Produce json
// @Success 200 {object} models.APIKeyListResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/api-keys [get]
func (h *APIKeyHandler) List(c *gin.Context) {
	keys, err := h.svc.List(c.Request.Context())
	if err != nil {
		logRequestError(h.logger, c, "list api keys failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list api keys"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": keys})
}

// Create godoc
// @Summary Create API key
// @Description Returns the secret once; it cannot be retrieved again.
// @Tags api-keys
// @Accept json
// @Produce json
// @Param payload body createAPIKeyRequest true "API key payload"
// @Success 201 {object} models.APIKeyCreateResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/api-keys [post]
func (h *APIKeyHandler) Create(c *gin.Context) {
	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	allowed, err := parseUUIDList(req.AllowedDiscoveryConfigIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid allowed_discovery_config_ids"})
		return
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil && strings.TrimSpace(*req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.ExpiresAt))
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid expires_at (use RFC3339)"})
			return
		}
		t := parsed.UTC()
		expiresAt = &t
	}
	resp, err := h.svc.Create(c.Request.Context(), services.CreateAPIKeyInput{
		Name:                      req.Name,
		Scopes:                    req.Scopes,
		AllowedDiscoveryConfigIDs: allowed,
		ExpiresAt:                 expiresAt,
	})
	if err != nil {
		if errors.Is(err, services.ErrAPIKeyNameRequired) ||
			errors.Is(err, services.ErrAPIKeyScopeRequired) ||
			errors.Is(err, services.ErrAPIKeyDiscoveryEmpty) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
			return
		}
		logRequestError(h.logger, c, "create api key failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create api key"})
		return
	}
	logAuditFailure(h.logger, c, "create", "api_key", resp.APIKey.ID.String(), h.audit.Record(c.Request.Context(), c.GetString("username"), "create", "api_key", resp.APIKey.ID.String(), gin.H{"name": resp.APIKey.Name}))
	c.JSON(http.StatusCreated, resp)
}

// Revoke godoc
// @Summary Revoke API key
// @Tags api-keys
// @Param id path string true "API key ID"
// @Success 204
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/api-keys/{id}/revoke [post]
func (h *APIKeyHandler) Revoke(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid api key id"})
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), id); err != nil {
		if errors.Is(err, services.ErrAPIKeyNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "api key not found"})
			return
		}
		logRequestError(h.logger, c, "revoke api key failed", err, zap.String("api_key_id", id.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to revoke api key"})
		return
	}
	logAuditFailure(h.logger, c, "revoke", "api_key", id.String(), h.audit.Record(c.Request.Context(), c.GetString("username"), "revoke", "api_key", id.String(), nil))
	c.Status(http.StatusNoContent)
}

// Delete godoc
// @Summary Delete API key
// @Tags api-keys
// @Param id path string true "API key ID"
// @Success 204
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/api-keys/{id} [delete]
func (h *APIKeyHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid api key id"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		logRequestError(h.logger, c, "delete api key failed", err, zap.String("api_key_id", id.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete api key"})
		return
	}
	logAuditFailure(h.logger, c, "delete", "api_key", id.String(), h.audit.Record(c.Request.Context(), c.GetString("username"), "delete", "api_key", id.String(), nil))
	c.Status(http.StatusNoContent)
}

func parseUUIDList(in []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(in))
	for _, raw := range in {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}
