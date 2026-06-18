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

type DomainProfileHandler struct {
	svc    *services.DomainProfileService
	audit  *services.AuditService
	logger *zap.Logger
}

func NewDomainProfileHandler(svc *services.DomainProfileService, audit *services.AuditService, logger *zap.Logger) *DomainProfileHandler {
	return &DomainProfileHandler{svc: svc, audit: audit, logger: nopLogger(logger)}
}

type domainProfileWriteRequest struct {
	Name        string                        `json:"name" binding:"required"`
	Description string                        `json:"description"`
	Bindings    []models.DomainProfileBinding `json:"bindings" binding:"required"`
}

// ListByDiscovery godoc
// @Summary List domain profiles for a discovery group
// @Tags domain-profiles
// @Produce json
// @Param id path string true "Discovery config ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/discovery/{id}/domain-profiles [get]
func (h *DomainProfileHandler) ListByDiscovery(c *gin.Context) {
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
		logRequestError(h.logger, c, "list domain profiles failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list domain profiles"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// Create godoc
// @Summary Create domain profile
// @Tags domain-profiles
// @Accept json
// @Produce json
// @Param id path string true "Discovery config ID"
// @Param payload body domainProfileWriteRequest true "Profile payload"
// @Success 201 {object} models.DomainProfile
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/discovery/{id}/domain-profiles [post]
func (h *DomainProfileHandler) Create(c *gin.Context) {
	discoveryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid discovery id"})
		return
	}
	var req domainProfileWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	profile, err := h.svc.Create(c.Request.Context(), services.CreateDomainProfileInput{
		DiscoveryConfigID: discoveryID,
		Name:              req.Name,
		Description:       req.Description,
		Bindings:          req.Bindings,
	})
	if err != nil {
		if respondDomainProfileWriteError(c, err) {
			return
		}
		logRequestError(h.logger, c, "create domain profile failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create domain profile"})
		return
	}
	logAuditFailure(h.logger, c, "create", "domain_profile", profile.ID.String(), h.audit.Record(c.Request.Context(), c.GetString("username"), "create", "domain_profile", profile.ID.String(), gin.H{"name": profile.Name, "discovery_config_id": profile.DiscoveryConfigID.String()}))
	c.JSON(http.StatusCreated, profile)
}

// Get godoc
// @Summary Get domain profile
// @Tags domain-profiles
// @Produce json
// @Param id path string true "Domain profile ID"
// @Success 200 {object} models.DomainProfile
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/domain-profiles/{id} [get]
func (h *DomainProfileHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid domain profile id"})
		return
	}
	profile, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrDomainProfileNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "domain profile not found"})
			return
		}
		logRequestError(h.logger, c, "get domain profile failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to load domain profile"})
		return
	}
	c.JSON(http.StatusOK, profile)
}

// Update godoc
// @Summary Update domain profile
// @Tags domain-profiles
// @Accept json
// @Produce json
// @Param id path string true "Domain profile ID"
// @Param payload body domainProfileWriteRequest true "Profile payload"
// @Success 200 {object} models.DomainProfile
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/domain-profiles/{id} [put]
func (h *DomainProfileHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid domain profile id"})
		return
	}
	var req domainProfileWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	profile, err := h.svc.Update(c.Request.Context(), id, services.UpdateDomainProfileInput{
		Name:        req.Name,
		Description: req.Description,
		Bindings:    req.Bindings,
	})
	if err != nil {
		if errors.Is(err, services.ErrDomainProfileNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "domain profile not found"})
			return
		}
		if respondDomainProfileWriteError(c, err) {
			return
		}
		logRequestError(h.logger, c, "update domain profile failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update domain profile"})
		return
	}
	logAuditFailure(h.logger, c, "update", "domain_profile", profile.ID.String(), h.audit.Record(c.Request.Context(), c.GetString("username"), "update", "domain_profile", profile.ID.String(), gin.H{"name": profile.Name}))
	c.JSON(http.StatusOK, profile)
}

// Delete godoc
// @Summary Delete domain profile
// @Tags domain-profiles
// @Param id path string true "Domain profile ID"
// @Success 204
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/domain-profiles/{id} [delete]
func (h *DomainProfileHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid domain profile id"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, services.ErrDomainProfileNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "domain profile not found"})
			return
		}
		logRequestError(h.logger, c, "delete domain profile failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete domain profile"})
		return
	}
	logAuditFailure(h.logger, c, "delete", "domain_profile", id.String(), h.audit.Record(c.Request.Context(), c.GetString("username"), "delete", "domain_profile", id.String(), nil))
	c.Status(http.StatusNoContent)
}

func respondDomainProfileWriteError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, services.ErrDiscoveryNotFound):
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "discovery config not found"})
	case errors.Is(err, services.ErrDomainProfileNameRequired),
		errors.Is(err, services.ErrDomainProfileBindingsEmpty),
		errors.Is(err, services.ErrDomainProfileInvalidBinding):
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
	case errors.Is(err, services.ErrDomainProfileNameTaken):
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: err.Error()})
	default:
		return false
	}
	return true
}
