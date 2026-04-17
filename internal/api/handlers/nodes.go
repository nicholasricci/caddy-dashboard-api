package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
)

type NodeHandler struct {
	svc   *services.NodeService
	audit *services.AuditService
}

type createNodeRequest struct {
	Name       string     `json:"name" binding:"required"`
	InstanceID *string    `json:"instance_id"`
	PrivateIP  *string    `json:"private_ip"`
	Region     string     `json:"region" binding:"required"`
	SSMEnabled *bool      `json:"ssm_enabled"`
	Status     *string    `json:"status"`
	LastSeenAt *time.Time `json:"last_seen_at"`
}

type updateNodeRequest struct {
	Name       *string    `json:"name"`
	InstanceID *string    `json:"instance_id"`
	PrivateIP  *string    `json:"private_ip"`
	Region     *string    `json:"region"`
	SSMEnabled *bool      `json:"ssm_enabled"`
	Status     *string    `json:"status"`
	LastSeenAt *time.Time `json:"last_seen_at"`
}

func NewNodeHandler(svc *services.NodeService, audit *services.AuditService) *NodeHandler {
	return &NodeHandler{svc: svc, audit: audit}
}

// List godoc
// @Summary List nodes
// @Description Returns all registered Caddy nodes
// @Tags nodes
// @Produce json
// @Success 200 {array} models.CaddyNode
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes [get]
func (h *NodeHandler) List(c *gin.Context) {
	limit, offset := parseLimitOffset(c)
	nodes, total, err := h.svc.ListPaginated(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list nodes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": nodes, "meta": models.PaginationMeta{Total: total, Limit: limit, Offset: offset}})
}

// Create godoc
// @Summary Create node
// @Description Creates a Caddy node manually (private IP or instance ID)
// @Tags nodes
// @Accept json
// @Produce json
// @Param payload body models.CaddyNode true "Node payload"
// @Success 201 {object} models.CaddyNode
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes [post]
func (h *NodeHandler) Create(c *gin.Context) {
	var req createNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	node := &models.CaddyNode{
		Name:       strings.TrimSpace(req.Name),
		InstanceID: req.InstanceID,
		PrivateIP:  req.PrivateIP,
		Region:     strings.TrimSpace(req.Region),
		LastSeenAt: req.LastSeenAt,
	}
	if req.SSMEnabled != nil {
		node.SSMEnabled = *req.SSMEnabled
	}
	if req.Status != nil {
		node.Status = strings.TrimSpace(*req.Status)
	}
	if err := h.svc.Create(c.Request.Context(), node); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create node"})
		return
	}
	_ = h.audit.Record(c.Request.Context(), c.GetString("username"), "create", "node", node.ID.String(), node)
	c.JSON(http.StatusCreated, node)
}

// Get godoc
// @Summary Get node
// @Description Returns a node by ID
// @Tags nodes
// @Produce json
// @Param id path string true "Node ID"
// @Success 200 {object} models.CaddyNode
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id} [get]
func (h *NodeHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	node, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrNodeNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "node not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to load node"})
		return
	}
	c.JSON(http.StatusOK, node)
}

// Update godoc
// @Summary Update node
// @Description Updates an existing node by ID
// @Tags nodes
// @Accept json
// @Produce json
// @Param id path string true "Node ID"
// @Param payload body models.CaddyNode true "Node payload"
// @Success 200 {object} models.CaddyNode
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id} [put]
func (h *NodeHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	node, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrNodeNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "node not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to load node"})
		return
	}
	var req updateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	if req.Name != nil {
		node.Name = strings.TrimSpace(*req.Name)
	}
	if req.InstanceID != nil {
		v := strings.TrimSpace(*req.InstanceID)
		if v == "" {
			node.InstanceID = nil
		} else {
			node.InstanceID = &v
		}
	}
	if req.PrivateIP != nil {
		v := strings.TrimSpace(*req.PrivateIP)
		if v == "" {
			node.PrivateIP = nil
		} else {
			node.PrivateIP = &v
		}
	}
	if req.Region != nil {
		node.Region = strings.TrimSpace(*req.Region)
	}
	if req.SSMEnabled != nil {
		node.SSMEnabled = *req.SSMEnabled
	}
	if req.Status != nil {
		node.Status = strings.TrimSpace(*req.Status)
	}
	if req.LastSeenAt != nil {
		node.LastSeenAt = req.LastSeenAt
	}
	node.ID = id
	if err := h.svc.Update(c.Request.Context(), node); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update node"})
		return
	}
	_ = h.audit.Record(c.Request.Context(), c.GetString("username"), "update", "node", node.ID.String(), req)
	c.JSON(http.StatusOK, node)
}

func parseLimitOffset(c *gin.Context) (int, int) {
	limit := 20
	offset := 0
	if v, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil {
		if v < 1 {
			v = 1
		}
		if v > 100 {
			v = 100
		}
		limit = v
	}
	if v, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil && v >= 0 {
		offset = v
	}
	return limit, offset
}

// Delete godoc
// @Summary Delete node
// @Description Deletes a node by ID
// @Tags nodes
// @Param id path string true "Node ID"
// @Success 204
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id} [delete]
func (h *NodeHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete node"})
		return
	}
	_ = h.audit.Record(c.Request.Context(), c.GetString("username"), "delete", "node", id.String(), nil)
	c.Status(http.StatusNoContent)
}
