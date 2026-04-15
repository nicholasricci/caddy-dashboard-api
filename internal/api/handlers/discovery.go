package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
)

type DiscoveryHandler struct {
	svc *services.DiscoveryService
}

func NewDiscoveryHandler(svc *services.DiscoveryService) *DiscoveryHandler {
	return &DiscoveryHandler{svc: svc}
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
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list discovery configs"})
		return
	}
	c.JSON(http.StatusOK, items)
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
	var req models.DiscoveryConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	if err := h.svc.Create(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create discovery config"})
		return
	}
	c.JSON(http.StatusCreated, req)
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
	var req models.DiscoveryConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	req.ID = id
	if err := h.svc.Update(c.Request.Context(), &req); err != nil {
		if errors.Is(err, services.ErrDiscoveryNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "discovery config not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update discovery config"})
		return
	}
	c.JSON(http.StatusOK, req)
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
	c.JSON(http.StatusOK, gin.H{"discovered_nodes": count})
}
