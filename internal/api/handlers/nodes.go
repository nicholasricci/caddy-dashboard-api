package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
)

type NodeHandler struct {
	svc *services.NodeService
}

func NewNodeHandler(svc *services.NodeService) *NodeHandler {
	return &NodeHandler{svc: svc}
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
	nodes, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list nodes"})
		return
	}
	c.JSON(http.StatusOK, nodes)
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
	var req models.CaddyNode
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	if err := h.svc.Create(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create node"})
		return
	}
	c.JSON(http.StatusCreated, req)
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
	if err := c.ShouldBindJSON(node); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	node.ID = id
	if err := h.svc.Update(c.Request.Context(), node); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update node"})
		return
	}
	c.JSON(http.StatusOK, node)
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
	c.Status(http.StatusNoContent)
}
