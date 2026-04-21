package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	caddysvc "github.com/nicholasricci/caddy-dashboard/internal/caddy"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
)

type CaddyHandler struct {
	svc   caddyService
	audit *services.AuditService
}

type caddyService interface {
	Sync(ctx context.Context, nodeID uuid.UUID, requestedBy string) error
	GetLiveConfig(ctx context.Context, nodeID uuid.UUID) (json.RawMessage, error)
	ListConfigIDs(ctx context.Context, nodeID uuid.UUID) ([]models.CaddyConfigIDInfo, error)
	GetConfigByID(ctx context.Context, nodeID uuid.UUID, configID string) (json.RawMessage, error)
	GetUpstreamsByID(ctx context.Context, nodeID uuid.UUID, configID string) ([]json.RawMessage, error)
	GetHostsByID(ctx context.Context, nodeID uuid.UUID, configID string) ([]string, error)
	Apply(ctx context.Context, nodeID uuid.UUID, payload json.RawMessage, requestedBy string) error
	Reload(ctx context.Context, nodeID uuid.UUID) error
	ListSnapshotsPaginated(ctx context.Context, nodeID uuid.UUID, limit, offset int) ([]models.CaddySnapshot, int64, error)
}

func NewCaddyHandler(svc caddyService, audit *services.AuditService) *CaddyHandler {
	return &CaddyHandler{svc: svc, audit: audit}
}

// respondCaddyNodeError writes the appropriate JSON error for node-scoped Caddy operations. Returns true if handled.
func respondCaddyNodeError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, models.ErrNodeNotFound) {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "node not found"})
		return true
	}
	if errors.Is(err, caddysvc.ErrNodeNoInstanceID) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "node has no instance_id configured"})
		return true
	}
	return false
}

type applyConfigRequest struct {
	Config json.RawMessage `json:"config" binding:"required" swaggertype:"object"`
}

// Sync godoc
// @Summary Sync node config
// @Description Fetches current Caddy config from node via SSM and stores a snapshot
// @Tags caddy
// @Produce json
// @Param id path string true "Node ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id}/sync [post]
func (h *CaddyHandler) Sync(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	username := c.GetString("username")
	if err := h.svc.Sync(c.Request.Context(), nodeID, username); err != nil {
		if respondCaddyNodeError(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "sync failed"})
		return
	}
	_ = h.audit.Record(c.Request.Context(), username, "sync", "node", nodeID.String(), nil)
	c.JSON(http.StatusOK, gin.H{"message": "node config synced"})
}

// LiveConfig godoc
// @Summary Get live Caddy config
// @Description Fetches current Caddy JSON config from the node via SSM (same as sync) without storing a snapshot
// @Tags caddy
// @Produce json
// @Param id path string true "Node ID"
// @Success 200 {object} object "Caddy admin API config JSON"
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id}/config/live [get]
func (h *CaddyHandler) LiveConfig(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	raw, err := h.svc.GetLiveConfig(c.Request.Context(), nodeID)
	if err != nil {
		if respondCaddyNodeError(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch live config"})
		return
	}
	c.Data(http.StatusOK, "application/json", raw)
}

// ListConfigIDs godoc
// @Summary List @id entries from live Caddy config
// @Description Fetches live Caddy config and returns all discovered @id entries, including upstream metadata when present
// @Tags caddy
// @Produce json
// @Param id path string true "Node ID"
// @Success 200 {object} models.CaddyConfigIDsResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id}/config/live/ids [get]
func (h *CaddyHandler) ListConfigIDs(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	items, err := h.svc.ListConfigIDs(c.Request.Context(), nodeID)
	if err != nil {
		if respondCaddyNodeError(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list config ids"})
		return
	}
	c.JSON(http.StatusOK, models.CaddyConfigIDsResponse{Items: items})
}

// ConfigByID godoc
// @Summary Get config fragment by @id
// @Description Fetches live Caddy config and returns the JSON fragment matching the requested @id
// @Tags caddy
// @Produce json
// @Param id path string true "Node ID"
// @Param configId path string true "@id value"
// @Success 200 {object} object "Config fragment JSON"
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id}/config/live/ids/{configId} [get]
func (h *CaddyHandler) ConfigByID(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	configID := strings.TrimSpace(c.Param("configId"))
	if configID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid config id"})
		return
	}
	raw, err := h.svc.GetConfigByID(c.Request.Context(), nodeID, configID)
	if err != nil {
		if respondCaddyNodeError(c, err) {
			return
		}
		if errors.Is(err, caddysvc.ErrConfigIDNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "config id not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch config fragment"})
		return
	}
	c.Data(http.StatusOK, "application/json", raw)
}

// UpstreamsByID godoc
// @Summary Get upstreams by @id
// @Description Fetches live Caddy config and returns upstreams associated with the requested @id
// @Tags caddy
// @Produce json
// @Param id path string true "Node ID"
// @Param configId path string true "@id value"
// @Success 200 {object} models.CaddyConfigUpstreamsResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id}/config/live/ids/{configId}/upstreams [get]
func (h *CaddyHandler) UpstreamsByID(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	configID := strings.TrimSpace(c.Param("configId"))
	if configID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid config id"})
		return
	}
	upstreams, err := h.svc.GetUpstreamsByID(c.Request.Context(), nodeID, configID)
	if err != nil {
		if respondCaddyNodeError(c, err) {
			return
		}
		if errors.Is(err, caddysvc.ErrConfigIDNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "config id not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch config upstreams"})
		return
	}
	c.JSON(http.StatusOK, models.CaddyConfigUpstreamsResponse{
		ID:            configID,
		HasUpstreams:  len(upstreams) > 0,
		UpstreamCount: len(upstreams),
		Upstreams:     rawMessagesToAny(upstreams),
	})
}

// HostsByID godoc
// @Summary Get hosts by @id
// @Description Fetches live Caddy config and returns unique hosts extracted from upstreams associated with the requested @id
// @Tags caddy
// @Produce json
// @Param id path string true "Node ID"
// @Param configId path string true "@id value"
// @Success 200 {object} models.CaddyConfigHostsResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id}/config/live/ids/{configId}/hosts [get]
func (h *CaddyHandler) HostsByID(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	configID := strings.TrimSpace(c.Param("configId"))
	if configID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid config id"})
		return
	}
	hosts, err := h.svc.GetHostsByID(c.Request.Context(), nodeID, configID)
	if err != nil {
		if respondCaddyNodeError(c, err) {
			return
		}
		if errors.Is(err, caddysvc.ErrConfigIDNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "config id not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch config hosts"})
		return
	}
	c.JSON(http.StatusOK, models.CaddyConfigHostsResponse{
		ID:        configID,
		HostCount: len(hosts),
		Hosts:     hosts,
	})
}

func rawMessagesToAny(values []json.RawMessage) []any {
	out := make([]any, 0, len(values))
	for _, raw := range values {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err == nil {
			out = append(out, decoded)
			continue
		}
		out = append(out, string(raw))
	}
	return out
}

// Apply godoc
// @Summary Apply Caddy config
// @Description Applies Caddy config to node via SSM and stores a snapshot
// @Tags caddy
// @Accept json
// @Produce json
// @Param id path string true "Node ID"
// @Param payload body applyConfigRequest true "Caddy config payload"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id}/apply [post]
func (h *CaddyHandler) Apply(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	var req applyConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	username := c.GetString("username")
	if err := h.svc.Apply(c.Request.Context(), nodeID, req.Config, username); err != nil {
		if respondCaddyNodeError(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "apply failed"})
		return
	}
	_ = h.audit.Record(c.Request.Context(), username, "apply", "node", nodeID.String(), gin.H{"config_size": len(req.Config)})
	c.JSON(http.StatusOK, gin.H{"message": "config applied"})
}

// Reload godoc
// @Summary Reload Caddy
// @Description Runs Caddy reload on node via SSM
// @Tags caddy
// @Produce json
// @Param id path string true "Node ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id}/reload [post]
func (h *CaddyHandler) Reload(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	if err := h.svc.Reload(c.Request.Context(), nodeID); err != nil {
		if respondCaddyNodeError(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "reload failed"})
		return
	}
	_ = h.audit.Record(c.Request.Context(), c.GetString("username"), "reload", "node", nodeID.String(), nil)
	c.JSON(http.StatusOK, gin.H{"message": "caddy reloaded"})
}

// ListSnapshots godoc
// @Summary List node snapshots
// @Description Returns stored Caddy configuration snapshots for a node
// @Tags caddy
// @Produce json
// @Param id path string true "Node ID"
// @Success 200 {array} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/nodes/{id}/snapshots [get]
func (h *CaddyHandler) ListSnapshots(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	limit, offset := parseLimitOffset(c)
	snapshots, total, err := h.svc.ListSnapshotsPaginated(c.Request.Context(), nodeID, limit, offset)
	if err != nil {
		if errors.Is(err, models.ErrNodeNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "node not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list snapshots"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": snapshots, "meta": models.PaginationMeta{Total: total, Limit: limit, Offset: offset}})
}
