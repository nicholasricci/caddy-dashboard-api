package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
)

type DiscoveryHandler struct {
	svc      *services.DiscoveryService
	caddySvc *services.CaddyService
	audit    *services.AuditService
}

type discoveryWriteRequest struct {
	Name          string          `json:"name" binding:"required"`
	Method        string          `json:"method"`
	Region        string          `json:"region" binding:"required"`
	TagKey        string          `json:"tag_key"`
	TagValue      string          `json:"tag_value"`
	Parameters    json.RawMessage `json:"parameters"`
	SnapshotScope string          `json:"snapshot_scope"`
	Enabled       *bool           `json:"enabled"`
}

func NewDiscoveryHandler(svc *services.DiscoveryService, caddySvc *services.CaddyService, audit *services.AuditService) *DiscoveryHandler {
	return &DiscoveryHandler{svc: svc, caddySvc: caddySvc, audit: audit}
}

// List godoc
// @Summary List discovery configs
// @Description Returns discovery rules
// @Tags discovery
// @Produce json
// @Success 200 {array} models.DiscoveryConfig
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/discovery [get]
func (h *DiscoveryHandler) List(c *gin.Context) {
	limit, offset := parseLimitOffset(c)
	items, total, err := h.svc.ListPaginated(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list discovery configs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "meta": models.PaginationMeta{Total: total, Limit: limit, Offset: offset}})
}

// Get godoc
// @Summary Get discovery config
// @Tags discovery
// @Produce json
// @Param id path string true "Discovery config ID"
// @Success 200 {object} models.DiscoveryConfig
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/discovery/{id} [get]
func (h *DiscoveryHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid discovery id"})
		return
	}
	cfg, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrDiscoveryNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "discovery config not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to load discovery config"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// Create godoc
// @Summary Create discovery config
// @Description Creates a discovery rule (methods: aws_tag, aws_ssm, static_ip; aws_cidr not implemented)
// @Tags discovery
// @Accept json
// @Produce json
// @Param payload body models.DiscoveryConfig true "Discovery config payload"
// @Success 201 {object} models.DiscoveryConfig
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/discovery [post]
func (h *DiscoveryHandler) Create(c *gin.Context) {
	var req discoveryWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	snapshotScope, err := models.ParseSnapshotScope(req.SnapshotScope)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "snapshot_scope must be one of: node, group"})
		return
	}
	cfg := models.DiscoveryConfig{
		Name:          strings.TrimSpace(req.Name),
		Method:        strings.TrimSpace(req.Method),
		Region:        strings.TrimSpace(req.Region),
		TagKey:        strings.TrimSpace(req.TagKey),
		TagValue:      strings.TrimSpace(req.TagValue),
		Parameters:    req.Parameters,
		SnapshotScope: snapshotScope,
	}
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	} else {
		cfg.Enabled = true
	}
	if err := h.svc.Create(c.Request.Context(), &cfg); err != nil {
		if errors.Is(err, services.ErrInvalidSnapshotScope) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "snapshot_scope must be one of: node, group"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create discovery config"})
		return
	}
	_ = h.audit.Record(c.Request.Context(), c.GetString("username"), "create", "discovery", cfg.ID.String(), cfg)
	c.JSON(http.StatusCreated, cfg)
}

// Update godoc
// @Summary Update discovery config
// @Tags discovery
// @Accept json
// @Produce json
// @Param id path string true "Discovery config ID"
// @Param payload body models.DiscoveryConfig true "Discovery config payload"
// @Success 200 {object} models.DiscoveryConfig
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/discovery/{id} [put]
func (h *DiscoveryHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid discovery id"})
		return
	}
	var req discoveryWriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	snapshotScope, err := models.ParseSnapshotScope(req.SnapshotScope)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "snapshot_scope must be one of: node, group"})
		return
	}
	cfg := models.DiscoveryConfig{
		ID:            id,
		Name:          strings.TrimSpace(req.Name),
		Method:        strings.TrimSpace(req.Method),
		Region:        strings.TrimSpace(req.Region),
		TagKey:        strings.TrimSpace(req.TagKey),
		TagValue:      strings.TrimSpace(req.TagValue),
		Parameters:    req.Parameters,
		SnapshotScope: snapshotScope,
	}
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	} else {
		cfg.Enabled = true
	}
	if err := h.svc.Update(c.Request.Context(), &cfg); err != nil {
		if errors.Is(err, services.ErrInvalidSnapshotScope) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "snapshot_scope must be one of: node, group"})
			return
		}
		if errors.Is(err, services.ErrDiscoveryNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "discovery config not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update discovery config"})
		return
	}
	_ = h.audit.Record(c.Request.Context(), c.GetString("username"), "update", "discovery", cfg.ID.String(), cfg)
	c.JSON(http.StatusOK, cfg)
}

// Delete godoc
// @Summary Delete discovery config
// @Tags discovery
// @Param id path string true "Discovery config ID"
// @Success 204
// @Failure 404 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/discovery/{id} [delete]
func (h *DiscoveryHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid discovery id"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, services.ErrDiscoveryNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "discovery config not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete discovery config"})
		return
	}
	_ = h.audit.Record(c.Request.Context(), c.GetString("username"), "delete", "discovery", id.String(), nil)
	c.Status(http.StatusNoContent)
}

// Run godoc
// @Summary Run discovery
// @Description Executes discovery for a specific discovery config and upserts found nodes
// @Tags discovery
// @Produce json
// @Param id path string true "Discovery config ID"
// @Success 200 {object} map[string]int
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 501 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/discovery/{id}/run [post]
func (h *DiscoveryHandler) Run(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid discovery id"})
		return
	}
	count, err := h.svc.Run(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrDiscoveryNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "discovery config not found"})
			return
		}
		if errors.Is(err, services.ErrDiscoveryMethodNotImplemented) {
			c.JSON(http.StatusNotImplemented, models.ErrorResponse{Error: "discovery method not implemented"})
			return
		}
		if errors.Is(err, services.ErrDiscoveryUnknownMethod) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "unknown discovery method"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "discovery run failed"})
		return
	}
	_ = h.audit.Record(c.Request.Context(), c.GetString("username"), "run", "discovery", id.String(), gin.H{"discovered_nodes": count})
	c.JSON(http.StatusOK, gin.H{"discovered_nodes": count})
}

// ListSnapshots godoc
// @Summary List discovery group snapshots
// @Description Returns stored Caddy snapshots for a discovery group
// @Tags discovery
// @Produce json
// @Param id path string true "Discovery config ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/discovery/{id}/snapshots [get]
func (h *DiscoveryHandler) ListSnapshots(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid discovery id"})
		return
	}
	if _, err := h.svc.Get(c.Request.Context(), id); err != nil {
		if errors.Is(err, services.ErrDiscoveryNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "discovery config not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to load discovery config"})
		return
	}
	limit, offset := parseLimitOffset(c)
	snapshots, total, err := h.caddySvc.ListDiscoverySnapshotsPaginated(c.Request.Context(), id, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list snapshots"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": snapshots, "meta": models.PaginationMeta{Total: total, Limit: limit, Offset: offset}})
}
