package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	caddysvc "github.com/nicholasricci/caddy-dashboard/internal/caddy"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
	"go.uber.org/zap"
)

type RegisterUpstreamHandler struct {
	svc     *services.RegisterUpstreamService
	apiKeys *services.APIKeyService
	audit   *services.AuditService
	logger  *zap.Logger
}

func NewRegisterUpstreamHandler(
	svc *services.RegisterUpstreamService,
	apiKeys *services.APIKeyService,
	audit *services.AuditService,
	logger *zap.Logger,
) *RegisterUpstreamHandler {
	return &RegisterUpstreamHandler{svc: svc, apiKeys: apiKeys, audit: audit, logger: nopLogger(logger)}
}

type registerUpstreamRequest struct {
	ConfigID  string `json:"config_id" binding:"required"`
	Port      int    `json:"port"`
	PrivateIP string `json:"private_ip"`
	Dial      string `json:"dial"`
	DryRun    bool   `json:"dry_run"`
}

// RegisterUpstream godoc
// @Summary Register upstream dial on a Caddy discovery group
// @Description Adds an upstream dial to the first reachable Caddy node in the group and propagates config to peers. Authenticated via API key.
// @Tags discovery
// @Accept json
// @Produce json
// @Param id path string true "Discovery config ID (Caddy proxy group)"
// @Param payload body registerUpstreamRequest true "Registration payload"
// @Success 200 {object} models.RegisterUpstreamResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Failure 422 {object} models.ErrorResponse
// @Failure 502 {object} models.ErrorResponse
// @Security APIKeyAuth
// @Router /api/v1/discovery/{id}/register-upstream [post]
func (h *RegisterUpstreamHandler) RegisterUpstream(c *gin.Context) {
	discoveryID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid discovery id"})
		return
	}
	validated, err := h.validatedAPIKey(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}
	if err := h.apiKeys.AuthorizeDiscovery(validated, discoveryID, models.APIKeyScopeRegisterUpstream); err != nil {
		if errors.Is(err, services.ErrAPIKeyForbidden) || errors.Is(err, services.ErrAPIKeyScopeMissing) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}
	var req registerUpstreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	requestedBy := "api_key:" + validated.Name
	resp, err := h.svc.Register(c.Request.Context(), services.RegisterUpstreamInput{
		DiscoveryConfigID: discoveryID,
		ConfigID:          req.ConfigID,
		Port:              req.Port,
		PrivateIP:         req.PrivateIP,
		Dial:              req.Dial,
		DryRun:            req.DryRun,
		RequestedBy:       requestedBy,
	})
	if err != nil {
		if respondRegisterUpstreamError(c, err) {
			return
		}
		logRequestError(h.logger, c, "register upstream failed", err, zap.String("discovery_id", discoveryID.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "register upstream failed"})
		return
	}
	if h.audit != nil {
		logAuditFailure(h.logger, c, "register_upstream", "discovery", discoveryID.String(), h.audit.Record(c.Request.Context(), requestedBy, "register_upstream", "discovery", discoveryID.String(), gin.H{
			"dial":           resp.Dial,
			"source_node_id": resp.SourceNodeID.String(),
			"changed":        resp.Changed,
		}))
	}
	c.JSON(http.StatusOK, toRegisterUpstreamResponse(resp))
}

func (h *RegisterUpstreamHandler) validatedAPIKey(c *gin.Context) (*services.ValidatedAPIKey, error) {
	name := c.GetString("api_key_name")
	if name == "" {
		return nil, services.ErrAPIKeyInvalid
	}
	idRaw := c.GetString("api_key_id")
	id, err := uuid.Parse(idRaw)
	if err != nil {
		return nil, services.ErrAPIKeyInvalid
	}
	scopes, _ := c.Get("api_key_scopes")
	scopeList, _ := scopes.([]string)
	allowedRaw, _ := c.Get("api_key_allowed_discovery_ids")
	allowed, _ := allowedRaw.([]uuid.UUID)
	return &services.ValidatedAPIKey{
		ID:                        id,
		Name:                      name,
		Scopes:                    scopeList,
		AllowedDiscoveryConfigIDs: allowed,
	}, nil
}

func respondRegisterUpstreamError(c *gin.Context, err error) bool {
	if errors.Is(err, services.ErrDiscoveryNotFound) {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "discovery config not found"})
		return true
	}
	if errors.Is(err, caddysvc.ErrConfigIDNotFound) {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "config id not found"})
		return true
	}
	if errors.Is(err, services.ErrInvalidRegisterDial) || errors.Is(err, caddysvc.ErrInvalidMutationPayload) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return true
	}
	if errors.Is(err, services.ErrNoOperationalNodes) || errors.Is(err, services.ErrNoReachableLeader) {
		c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: err.Error()})
		return true
	}
	if errors.Is(err, services.ErrRegisterLockTimeout) {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: err.Error()})
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		c.JSON(http.StatusGatewayTimeout, models.ErrorResponse{Error: "remote operation timed out"})
		return true
	}
	if errors.Is(err, caddysvc.ErrTransportUnsupportedOp) || errors.Is(err, caddysvc.ErrTransportNotConfigured) || errors.Is(err, caddysvc.ErrNodeNoInstanceID) {
		c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: err.Error()})
		return true
	}
	if errors.Is(err, caddysvc.ErrTransportUnreachable) {
		c.JSON(http.StatusBadGateway, models.ErrorResponse{Error: err.Error()})
		return true
	}
	return false
}

func toRegisterUpstreamResponse(in *services.RegisterUpstreamResult) models.RegisterUpstreamResponse {
	out := models.RegisterUpstreamResponse{
		DiscoveryConfigID: in.DiscoveryConfigID.String(),
		SourceNodeID:      in.SourceNodeID.String(),
		Dial:              in.Dial,
		Changed:           in.Changed,
		DryRun:            in.DryRun,
	}
	if in.Mutate != nil {
		mutate := models.MutateUpstreamsResponse{
			Changed: in.Mutate.Changed,
			DryRun:  in.Mutate.DryRun,
			Diff: models.UpstreamMutationDiff{
				Added:   in.Mutate.Diff.Added,
				Removed: in.Mutate.Diff.Removed,
				Pruned:  in.Mutate.Diff.Pruned,
			},
		}
		for _, item := range in.Mutate.Results {
			mutate.Results = append(mutate.Results, models.UpstreamMutationResult{
				ConfigID:  item.ConfigID,
				Upstreams: item.Upstreams,
				Pruned:    item.Pruned,
				Changed:   item.Changed,
				Added:     item.Added,
				Removed:   item.Removed,
			})
		}
		out.Mutate = &mutate
	}
	if in.Propagate != nil {
		prop := models.PropagateConfigResponse{SourceNodeID: in.Propagate.SourceNodeID.String()}
		for _, id := range in.Propagate.AppliedTo {
			prop.AppliedTo = append(prop.AppliedTo, id.String())
		}
		for _, id := range in.Propagate.Skipped {
			prop.Skipped = append(prop.Skipped, id.String())
		}
		out.Propagate = &prop
	}
	return out
}
