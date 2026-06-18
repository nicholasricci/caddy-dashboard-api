package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
	"go.uber.org/zap"
)

type UpstreamProfileHandler struct {
	svc    *services.UpstreamProfileService
	audit  *services.AuditService
	logger *zap.Logger
}

func NewUpstreamProfileHandler(svc *services.UpstreamProfileService, audit *services.AuditService, logger *zap.Logger) *UpstreamProfileHandler {
	return &UpstreamProfileHandler{svc: svc, audit: audit, logger: nopLogger(logger)}
}

type upstreamProfileWriteRequest struct {
	Name        string                          `json:"name" binding:"required"`
	Description string                          `json:"description"`
	Bindings    []models.UpstreamProfileBinding `json:"bindings" binding:"required"`
}

// ListByDiscovery godoc
// @Summary List upstream profiles for a discovery group
// @Tags upstream-profiles
// @Produce json
// @Param id path string true "Discovery config ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/discovery/{id}/upstream-profiles [get]
func (h *UpstreamProfileHandler) ListByDiscovery(c *gin.Context) {
	discoveryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid discovery id"})
		return
	}
	items, err := h.svc.ListByDiscovery(c.Request.Context(), discoveryID)
	if err != nil {
		if errors.Is(err, services.ErrDiscoveryNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "discovery config not found"})
			return
		}
		logRequestError(h.logger, c, "list upstream profiles failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list upstream profiles"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// Create godoc
// @Summary Create upstream profile
// @Tags upstream-profiles
// @Accept json
// @Produce json
// @Param id path string true "Discovery config ID"
// @Param payload body upstreamProfileWriteRequest true "Profile payload"
// @Success 201 {object} models.UpstreamProfile
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/discovery/{id}/upstream-profiles [post]
func (h *UpstreamProfileHandler) Create(c *gin.Context) {
	discoveryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid discovery id"})
		return
	}
	var req upstreamProfileWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	profile, err := h.svc.Create(c.Request.Context(), services.CreateUpstreamProfileInput{
		DiscoveryConfigID: discoveryID,
		Name:              req.Name,
		Description:       req.Description,
		Bindings:          req.Bindings,
	})
	if err != nil {
		if respondUpstreamProfileWriteError(c, err) {
			return
		}
		logRequestError(h.logger, c, "create upstream profile failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create upstream profile"})
		return
	}
	logAuditFailure(h.logger, c, "create", "upstream_profile", profile.ID.String(), h.audit.Record(c.Request.Context(), c.GetString("username"), "create", "upstream_profile", profile.ID.String(), gin.H{"name": profile.Name, "discovery_config_id": profile.DiscoveryConfigID.String()}))
	c.JSON(http.StatusCreated, profile)
}

// Get godoc
// @Summary Get upstream profile
// @Tags upstream-profiles
// @Produce json
// @Param id path string true "Upstream profile ID"
// @Success 200 {object} models.UpstreamProfile
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/upstream-profiles/{id} [get]
func (h *UpstreamProfileHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid upstream profile id"})
		return
	}
	profile, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrUpstreamProfileNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "upstream profile not found"})
			return
		}
		logRequestError(h.logger, c, "get upstream profile failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to load upstream profile"})
		return
	}
	c.JSON(http.StatusOK, profile)
}

// Update godoc
// @Summary Update upstream profile
// @Tags upstream-profiles
// @Accept json
// @Produce json
// @Param id path string true "Upstream profile ID"
// @Param payload body upstreamProfileWriteRequest true "Profile payload"
// @Success 200 {object} models.UpstreamProfile
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/upstream-profiles/{id} [put]
func (h *UpstreamProfileHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid upstream profile id"})
		return
	}
	var req upstreamProfileWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	profile, err := h.svc.Update(c.Request.Context(), id, services.UpdateUpstreamProfileInput{
		Name:        req.Name,
		Description: req.Description,
		Bindings:    req.Bindings,
	})
	if err != nil {
		if errors.Is(err, services.ErrUpstreamProfileNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "upstream profile not found"})
			return
		}
		if respondUpstreamProfileWriteError(c, err) {
			return
		}
		logRequestError(h.logger, c, "update upstream profile failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update upstream profile"})
		return
	}
	logAuditFailure(h.logger, c, "update", "upstream_profile", profile.ID.String(), h.audit.Record(c.Request.Context(), c.GetString("username"), "update", "upstream_profile", profile.ID.String(), gin.H{"name": profile.Name}))
	c.JSON(http.StatusOK, profile)
}

// Delete godoc
// @Summary Delete upstream profile
// @Tags upstream-profiles
// @Param id path string true "Upstream profile ID"
// @Success 204
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/upstream-profiles/{id} [delete]
func (h *UpstreamProfileHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid upstream profile id"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, services.ErrUpstreamProfileNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "upstream profile not found"})
			return
		}
		logRequestError(h.logger, c, "delete upstream profile failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete upstream profile"})
		return
	}
	logAuditFailure(h.logger, c, "delete", "upstream_profile", id.String(), h.audit.Record(c.Request.Context(), c.GetString("username"), "delete", "upstream_profile", id.String(), nil))
	c.Status(http.StatusNoContent)
}

func respondUpstreamProfileWriteError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, services.ErrDiscoveryNotFound):
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "discovery config not found"})
	case errors.Is(err, services.ErrUpstreamProfileNameRequired),
		errors.Is(err, services.ErrUpstreamProfileBindingsEmpty),
		errors.Is(err, services.ErrUpstreamProfileInvalidBinding):
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
	case errors.Is(err, services.ErrUpstreamProfileNameTaken):
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: err.Error()})
	default:
		return false
	}
	return true
}
