package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	caddysvc "github.com/nicholasricci/caddy-dashboard/internal/caddy"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
	"go.uber.org/zap"
)

type CaddyHandler struct {
	svc    caddyService
	audit  *services.AuditService
	logger *zap.Logger
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
	MutateDomains(ctx context.Context, nodeID uuid.UUID, req caddysvc.MutateDomainsRequest, requestedBy string) (*caddysvc.MutateDomainsResponse, error)
	MutateUpstreams(ctx context.Context, nodeID uuid.UUID, req caddysvc.MutateUpstreamsRequest, requestedBy string) (*caddysvc.MutateUpstreamsResponse, error)
	PropagateToDiscoveryPeers(ctx context.Context, sourceNodeID uuid.UUID, requestedBy string) (*caddysvc.PropagateResponse, error)
	ListSnapshotsPaginated(ctx context.Context, nodeID uuid.UUID, limit, offset int) ([]models.CaddySnapshot, int64, error)
}

func NewCaddyHandler(svc caddyService, audit *services.AuditService, logger *zap.Logger) *CaddyHandler {
	return &CaddyHandler{svc: svc, audit: audit, logger: nopLogger(logger)}
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
	if errors.Is(err, context.DeadlineExceeded) {
		c.JSON(http.StatusGatewayTimeout, models.ErrorResponse{Error: "remote operation timed out"})
		return true
	}
	if errors.Is(err, caddysvc.ErrTransportUnsupportedOp) {
		c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: err.Error()})
		return true
	}
	if errors.Is(err, caddysvc.ErrTransportNotConfigured) || errors.Is(err, caddysvc.ErrNodeNoInstanceID) {
		c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: err.Error()})
		return true
	}
	if errors.Is(err, caddysvc.ErrTransportUnreachable) {
		c.JSON(http.StatusBadGateway, models.ErrorResponse{Error: err.Error()})
		return true
	}
	if errors.Is(err, caddysvc.ErrUnknownTransport) {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return true
	}
	return false
}

type applyConfigRequest struct {
	Config json.RawMessage `json:"config" binding:"required" swaggertype:"object"`
}

type domainMutationTargetRequest struct {
	ConfigID      string   `json:"config_id" binding:"required"`
	MatchIndexes  []int    `json:"match_indexes"`
	AddDomains    []string `json:"add_domains"`
	RemoveDomains []string `json:"remove_domains"`
}

type dnsChallengeRequest struct {
	Provider string `json:"provider"`
	APIToken string `json:"api_token"`
}

type mutateDomainsRequest struct {
	Targets           []domainMutationTargetRequest `json:"targets" binding:"required"`
	UpdateTLSPolicies bool                          `json:"update_tls_policies"`
	DNSChallenge      *dnsChallengeRequest          `json:"dns_challenge,omitempty"`
	DryRun            bool                          `json:"dry_run"`
}

type upstreamMutationTargetRequest struct {
	ConfigID       string `json:"config_id" binding:"required"`
	AddDial        string `json:"add_dial"`
	RemoveDial     string `json:"remove_dial"`
	PruneUnhealthy bool   `json:"prune_unhealthy"`
	ProbeTimeoutMs int    `json:"probe_timeout_ms"`
}

type mutateUpstreamsRequest struct {
	Targets []upstreamMutationTargetRequest `json:"targets" binding:"required"`
	DryRun  bool                            `json:"dry_run"`
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
		logRequestError(h.logger, c, "caddy sync failed", err, zap.String("node_id", nodeID.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "sync failed"})
		return
	}
	logAuditFailure(h.logger, c, "sync", "node", nodeID.String(), h.audit.Record(c.Request.Context(), username, "sync", "node", nodeID.String(), nil))
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
		logRequestError(h.logger, c, "fetch live config failed", err, zap.String("node_id", nodeID.String()))
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
		logRequestError(h.logger, c, "list config ids failed", err, zap.String("node_id", nodeID.String()))
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
		logRequestError(h.logger, c, "fetch config fragment failed", err,
			zap.String("node_id", nodeID.String()),
			zap.String("config_id", configID),
		)
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
		logRequestError(h.logger, c, "fetch config upstreams failed", err,
			zap.String("node_id", nodeID.String()),
			zap.String("config_id", configID),
		)
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
		logRequestError(h.logger, c, "fetch config hosts failed", err,
			zap.String("node_id", nodeID.String()),
			zap.String("config_id", configID),
		)
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
		logRequestError(h.logger, c, "caddy apply failed", err, zap.String("node_id", nodeID.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "apply failed"})
		return
	}
	logAuditFailure(h.logger, c, "apply", "node", nodeID.String(), h.audit.Record(c.Request.Context(), username, "apply", "node", nodeID.String(), gin.H{"config_size": len(req.Config)}))
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
		logRequestError(h.logger, c, "caddy reload failed", err, zap.String("node_id", nodeID.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "reload failed"})
		return
	}
	logAuditFailure(h.logger, c, "reload", "node", nodeID.String(), h.audit.Record(c.Request.Context(), c.GetString("username"), "reload", "node", nodeID.String(), nil))
	c.JSON(http.StatusOK, gin.H{"message": "caddy reloaded"})
}

// MutateDomains godoc
// @Summary Mutate domains by config @id
// @Description Adds/removes host domains on one or more config fragments by @id and optional match indexes, then applies updated config
// @Tags caddy
// @Accept json
// @Produce json
// @Param id path string true "Node ID"
// @Param payload body mutateDomainsRequest true "Domain mutation request"
// @Success 200 {object} models.MutateDomainsResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 422 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id}/config/mutate/domains [post]
func (h *CaddyHandler) MutateDomains(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	var req mutateDomainsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	targets := make([]caddysvc.DomainMutationTarget, 0, len(req.Targets))
	for _, t := range req.Targets {
		targets = append(targets, caddysvc.DomainMutationTarget{
			ConfigID:      strings.TrimSpace(t.ConfigID),
			MatchIndexes:  t.MatchIndexes,
			AddDomains:    t.AddDomains,
			RemoveDomains: t.RemoveDomains,
		})
	}
	var challenge *caddysvc.TLSDNSChallenge
	if req.DNSChallenge != nil {
		challenge = &caddysvc.TLSDNSChallenge{
			Provider: strings.TrimSpace(req.DNSChallenge.Provider),
			APIToken: strings.TrimSpace(req.DNSChallenge.APIToken),
		}
	}
	resp, err := h.svc.MutateDomains(c.Request.Context(), nodeID, caddysvc.MutateDomainsRequest{
		Targets:           targets,
		UpdateTLSPolicies: req.UpdateTLSPolicies,
		TLSDNSChallenge:   challenge,
		DryRun:            req.DryRun,
	}, c.GetString("username"))
	if err != nil {
		if respondCaddyNodeError(c, err) {
			return
		}
		if errors.Is(err, caddysvc.ErrConfigIDNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "config id not found"})
			return
		}
		if errors.Is(err, caddysvc.ErrInvalidMutationPayload) || errors.Is(err, caddysvc.ErrConfigIDShapeMismatch) {
			c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: err.Error()})
			return
		}
		logRequestError(h.logger, c, "caddy mutate domains failed", err, zap.String("node_id", nodeID.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "domain mutation failed"})
		return
	}
	if h.audit != nil {
		logAuditFailure(h.logger, c, "mutate_domains", "node", nodeID.String(), h.audit.Record(c.Request.Context(), c.GetString("username"), "mutate_domains", "node", nodeID.String(), gin.H{"targets": len(req.Targets)}))
	}
	modelResp := models.MutateDomainsResponse{
		Results: make([]models.DomainMutationResult, 0, len(resp.Results)),
		Changed: resp.Changed,
		DryRun:  resp.DryRun,
		Diff: models.DomainMutationDiff{
			Added:   resp.Diff.Added,
			Removed: resp.Diff.Removed,
		},
		Preview: rawPreviewToAny(resp.Preview),
	}
	for _, item := range resp.Results {
		modelResp.Results = append(modelResp.Results, models.DomainMutationResult{
			ConfigID: item.ConfigID,
			Hosts:    item.Hosts,
			Changed:  item.Changed,
			Added:    item.Added,
			Removed:  item.Removed,
		})
	}
	c.JSON(http.StatusOK, modelResp)
}

// MutateUpstreams godoc
// @Summary Mutate upstream dials by config @id
// @Description Adds/removes/prunes upstream dials on one or more config fragments by @id, then applies updated config
// @Tags caddy
// @Accept json
// @Produce json
// @Param id path string true "Node ID"
// @Param payload body mutateUpstreamsRequest true "Upstream mutation request"
// @Success 200 {object} models.MutateUpstreamsResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 422 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id}/config/mutate/upstreams [post]
func (h *CaddyHandler) MutateUpstreams(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	var req mutateUpstreamsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	targets := make([]caddysvc.UpstreamMutationTarget, 0, len(req.Targets))
	for _, t := range req.Targets {
		timeout := t.ProbeTimeoutMs
		if timeout <= 0 {
			timeout = 500
		}
		targets = append(targets, caddysvc.UpstreamMutationTarget{
			ConfigID:        strings.TrimSpace(t.ConfigID),
			AddDial:         strings.TrimSpace(t.AddDial),
			RemoveDial:      strings.TrimSpace(t.RemoveDial),
			PruneUnhealthy:  t.PruneUnhealthy,
			ProbeTimeout:    time.Duration(timeout) * time.Millisecond,
		})
	}
	resp, err := h.svc.MutateUpstreams(c.Request.Context(), nodeID, caddysvc.MutateUpstreamsRequest{
		Targets: targets,
		DryRun:  req.DryRun,
	}, c.GetString("username"))
	if err != nil {
		if respondCaddyNodeError(c, err) {
			return
		}
		if errors.Is(err, caddysvc.ErrConfigIDNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "config id not found"})
			return
		}
		if errors.Is(err, caddysvc.ErrInvalidMutationPayload) || errors.Is(err, caddysvc.ErrConfigIDShapeMismatch) {
			c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: err.Error()})
			return
		}
		logRequestError(h.logger, c, "caddy mutate upstreams failed", err, zap.String("node_id", nodeID.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "upstream mutation failed"})
		return
	}
	if h.audit != nil {
		logAuditFailure(h.logger, c, "mutate_upstreams", "node", nodeID.String(), h.audit.Record(c.Request.Context(), c.GetString("username"), "mutate_upstreams", "node", nodeID.String(), gin.H{"targets": len(req.Targets)}))
	}
	modelResp := models.MutateUpstreamsResponse{
		Results: make([]models.UpstreamMutationResult, 0, len(resp.Results)),
		Changed: resp.Changed,
		DryRun:  resp.DryRun,
		Diff: models.UpstreamMutationDiff{
			Added:   resp.Diff.Added,
			Removed: resp.Diff.Removed,
			Pruned:  resp.Diff.Pruned,
		},
		Preview: rawPreviewToAny(resp.Preview),
	}
	for _, item := range resp.Results {
		modelResp.Results = append(modelResp.Results, models.UpstreamMutationResult{
			ConfigID:  item.ConfigID,
			Upstreams: item.Upstreams,
			Pruned:    item.Pruned,
			Changed:   item.Changed,
			Added:     item.Added,
			Removed:   item.Removed,
		})
	}
	c.JSON(http.StatusOK, modelResp)
}

func rawPreviewToAny(in map[string]json.RawMessage) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, raw := range in {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err == nil {
			out[key] = decoded
			continue
		}
		out[key] = string(raw)
	}
	return out
}

// PropagateConfig godoc
// @Summary Propagate source node config to discovery peers
// @Description Fetches live config from source node and applies it to peer nodes in the same discovery config
// @Tags caddy
// @Produce json
// @Param id path string true "Source Node ID"
// @Success 200 {object} models.PropagateConfigResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/nodes/{id}/config/propagate [post]
func (h *CaddyHandler) PropagateConfig(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid node id"})
		return
	}
	resp, err := h.svc.PropagateToDiscoveryPeers(c.Request.Context(), nodeID, c.GetString("username"))
	if err != nil {
		if respondCaddyNodeError(c, err) {
			return
		}
		logRequestError(h.logger, c, "caddy propagate failed", err, zap.String("node_id", nodeID.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "propagate failed"})
		return
	}
	if h.audit != nil {
		logAuditFailure(h.logger, c, "propagate", "node", nodeID.String(), h.audit.Record(c.Request.Context(), c.GetString("username"), "propagate", "node", nodeID.String(), gin.H{"applied_to": len(resp.AppliedTo), "skipped": len(resp.Skipped)}))
	}
	modelResp := models.PropagateConfigResponse{
		SourceNodeID: resp.SourceNodeID.String(),
		AppliedTo:    make([]string, 0, len(resp.AppliedTo)),
		Skipped:      make([]string, 0, len(resp.Skipped)),
	}
	for _, id := range resp.AppliedTo {
		modelResp.AppliedTo = append(modelResp.AppliedTo, id.String())
	}
	for _, id := range resp.Skipped {
		modelResp.Skipped = append(modelResp.Skipped, id.String())
	}
	c.JSON(http.StatusOK, modelResp)
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
		logRequestError(h.logger, c, "list node snapshots failed", err, zap.String("node_id", nodeID.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list snapshots"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": snapshots, "meta": models.PaginationMeta{Total: total, Limit: limit, Offset: offset}})
}
