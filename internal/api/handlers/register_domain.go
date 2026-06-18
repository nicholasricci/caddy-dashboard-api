package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	caddysvc "github.com/nicholasricci/caddy-dashboard/internal/caddy"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
	"go.uber.org/zap"
)

type RegisterDomainHandler struct {
	svc     *services.RegisterDomainService
	apiKeys *services.APIKeyService
	audit   *services.AuditService
	logger  *zap.Logger
}

func NewRegisterDomainHandler(
	svc *services.RegisterDomainService,
	apiKeys *services.APIKeyService,
	audit *services.AuditService,
	logger *zap.Logger,
) *RegisterDomainHandler {
	return &RegisterDomainHandler{svc: svc, apiKeys: apiKeys, audit: audit, logger: nopLogger(logger)}
}

type registerDomainTLSRequest struct {
	UpdateTLSPolicies bool                 `json:"update_tls_policies"`
	DNSChallenge      *dnsChallengeRequest `json:"dns_challenge,omitempty"`
}

type registerDomainRequest struct {
	ConfigID     string   `json:"config_id" binding:"required"`
	MatchIndexes []int    `json:"match_indexes"`
	Domains      []string `json:"domains" binding:"required"`
	registerDomainTLSRequest
	DryRun bool `json:"dry_run"`
}

type registerDomainByProfileRequest struct {
	Domains []string `json:"domains" binding:"required"`
	registerDomainTLSRequest
	DryRun bool `json:"dry_run"`
}

// RegisterDomain godoc
// @Summary Register domains on a Caddy discovery group
// @Description Adds hostnames to the first reachable Caddy node in the group and propagates config to peers. Authenticated via API key.
// @Tags discovery
// @Accept json
// @Produce json
// @Param id path string true "Discovery config ID (Caddy proxy group)"
// @Param payload body registerDomainRequest true "Registration payload"
// @Success 200 {object} models.RegisterDomainResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Failure 422 {object} models.ErrorResponse
// @Failure 502 {object} models.ErrorResponse
// @Security APIKeyAuth
// @Router /api/v1/discovery/{id}/register-domain [post]
func (h *RegisterDomainHandler) RegisterDomain(c *gin.Context) {
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
	if err := h.apiKeys.AuthorizeDiscovery(validated, discoveryID, models.APIKeyScopeRegisterDomain); err != nil {
		if errors.Is(err, services.ErrAPIKeyForbidden) || errors.Is(err, services.ErrAPIKeyScopeMissing) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}
	var req registerDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	requestedBy := "api_key:" + validated.Name
	resp, err := h.svc.Register(c.Request.Context(), services.RegisterDomainInput{
		DiscoveryConfigID: discoveryID,
		ConfigID:          req.ConfigID,
		MatchIndexes:      req.MatchIndexes,
		Domains:           req.Domains,
		TLS:               toRegisterDomainTLSInput(req.registerDomainTLSRequest),
		DryRun:            req.DryRun,
		RequestedBy:       requestedBy,
	})
	if err != nil {
		if respondRegisterDomainError(c, err) {
			return
		}
		logRequestError(h.logger, c, "register domain failed", err, zap.String("discovery_id", discoveryID.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "register domain failed"})
		return
	}
	if h.audit != nil {
		logAuditFailure(h.logger, c, "register_domain", "discovery", discoveryID.String(), h.audit.Record(c.Request.Context(), requestedBy, "register_domain", "discovery", discoveryID.String(), gin.H{
			"domains":        resp.Domains,
			"source_node_id": resp.SourceNodeID.String(),
			"changed":        resp.Changed,
		}))
	}
	c.JSON(http.StatusOK, toRegisterDomainResponse(resp))
}

// RegisterDomainByProfile godoc
// @Summary Register domains using a domain profile
// @Description Adds hostnames for all bindings in the profile on the first reachable Caddy node and propagates to peers. Authenticated via API key.
// @Tags domain-profiles
// @Accept json
// @Produce json
// @Param id path string true "Domain profile ID"
// @Param payload body registerDomainByProfileRequest true "Registration payload"
// @Success 200 {object} models.RegisterDomainProfileResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Failure 422 {object} models.ErrorResponse
// @Failure 502 {object} models.ErrorResponse
// @Security APIKeyAuth
// @Router /api/v1/domain-profiles/{id}/register [post]
func (h *RegisterDomainHandler) RegisterDomainByProfile(c *gin.Context) {
	profileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid domain profile id"})
		return
	}
	validated, err := h.validatedAPIKey(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}
	profile, err := h.svc.GetProfile(c.Request.Context(), profileID)
	if err != nil {
		if errors.Is(err, services.ErrDomainProfileNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "domain profile not found"})
			return
		}
		logRequestError(h.logger, c, "load domain profile failed", err, zap.String("profile_id", profileID.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to load domain profile"})
		return
	}
	if err := h.apiKeys.AuthorizeDomainProfile(validated, profileID, profile.DiscoveryConfigID, models.APIKeyScopeRegisterDomain); err != nil {
		if errors.Is(err, services.ErrAPIKeyForbidden) || errors.Is(err, services.ErrAPIKeyDomainProfileForbidden) || errors.Is(err, services.ErrAPIKeyScopeMissing) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}
	var req registerDomainByProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	requestedBy := "api_key:" + validated.Name
	resp, err := h.svc.RegisterByProfile(c.Request.Context(), services.RegisterDomainByProfileInput{
		ProfileID:   profileID,
		Domains:     req.Domains,
		TLS:         toRegisterDomainTLSInput(req.registerDomainTLSRequest),
		DryRun:      req.DryRun,
		RequestedBy: requestedBy,
	})
	if err != nil {
		if errors.Is(err, services.ErrDomainProfileNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "domain profile not found"})
			return
		}
		if respondRegisterDomainError(c, err) {
			return
		}
		logRequestError(h.logger, c, "register domain by profile failed", err, zap.String("profile_id", profileID.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "register domain by profile failed"})
		return
	}
	if h.audit != nil {
		targets := make([]gin.H, 0, len(resp.Targets))
		for _, t := range resp.Targets {
			targets = append(targets, gin.H{"config_id": t.ConfigID, "match_indexes": t.MatchIndexes, "domains": t.Domains})
		}
		logAuditFailure(h.logger, c, "register_domain_profile", "domain_profile", profileID.String(), h.audit.Record(c.Request.Context(), requestedBy, "register_domain_profile", "domain_profile", profileID.String(), gin.H{
			"targets":        targets,
			"source_node_id": resp.SourceNodeID.String(),
			"changed":        resp.Changed,
		}))
	}
	c.JSON(http.StatusOK, toRegisterDomainProfileResponse(profileID, resp))
}

func (h *RegisterDomainHandler) validatedAPIKey(c *gin.Context) (*services.ValidatedAPIKey, error) {
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
	profilesRaw, _ := c.Get("api_key_allowed_upstream_profile_ids")
	profiles, _ := profilesRaw.([]uuid.UUID)
	domainProfilesRaw, _ := c.Get("api_key_allowed_domain_profile_ids")
	domainProfiles, _ := domainProfilesRaw.([]uuid.UUID)
	return &services.ValidatedAPIKey{
		ID:                        id,
		Name:                      name,
		Scopes:                    scopeList,
		AllowedDiscoveryConfigIDs: allowed,
		AllowedUpstreamProfileIDs: profiles,
		AllowedDomainProfileIDs:   domainProfiles,
	}, nil
}

func toRegisterDomainTLSInput(req registerDomainTLSRequest) services.RegisterDomainTLSInput {
	out := services.RegisterDomainTLSInput{UpdateTLSPolicies: req.UpdateTLSPolicies}
	if req.DNSChallenge != nil {
		out.DNSChallenge = &caddysvc.TLSDNSChallenge{
			Provider: strings.TrimSpace(req.DNSChallenge.Provider),
			APIToken: strings.TrimSpace(req.DNSChallenge.APIToken),
		}
	}
	return out
}

func respondRegisterDomainError(c *gin.Context, err error) bool {
	if errors.Is(err, services.ErrDiscoveryNotFound) {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "discovery config not found"})
		return true
	}
	if errors.Is(err, caddysvc.ErrConfigIDNotFound) {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "config id not found"})
		return true
	}
	if errors.Is(err, services.ErrInvalidRegisterDomains) || errors.Is(err, caddysvc.ErrInvalidMutationPayload) {
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

func toRegisterDomainResponse(in *services.RegisterDomainResult) models.RegisterDomainResponse {
	out := models.RegisterDomainResponse{
		DiscoveryConfigID: in.DiscoveryConfigID.String(),
		SourceNodeID:      in.SourceNodeID.String(),
		Domains:           in.Domains,
		Changed:           in.Changed,
		DryRun:            in.DryRun,
	}
	if in.Mutate != nil {
		out.Mutate = toModelMutateDomainsResponse(in.Mutate)
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

func toRegisterDomainProfileResponse(profileID uuid.UUID, in *services.RegisterDomainResult) models.RegisterDomainProfileResponse {
	out := models.RegisterDomainProfileResponse{
		DomainProfileID:   profileID.String(),
		DiscoveryConfigID: in.DiscoveryConfigID.String(),
		SourceNodeID:      in.SourceNodeID.String(),
		Changed:           in.Changed,
		DryRun:            in.DryRun,
	}
	for _, t := range in.Targets {
		out.Targets = append(out.Targets, models.RegisterDomainProfileTarget{
			ConfigID:     t.ConfigID,
			MatchIndexes: t.MatchIndexes,
			Domains:      t.Domains,
		})
	}
	if in.Mutate != nil {
		out.Mutate = toModelMutateDomainsResponse(in.Mutate)
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

func toModelMutateDomainsResponse(in *caddysvc.MutateDomainsResponse) *models.MutateDomainsResponse {
	mutate := models.MutateDomainsResponse{
		Changed: in.Changed,
		DryRun:  in.DryRun,
		Diff: models.DomainMutationDiff{
			Added:   in.Diff.Added,
			Removed: in.Diff.Removed,
		},
		Preview: rawPreviewToAny(in.Preview),
	}
	for _, item := range in.Results {
		mutate.Results = append(mutate.Results, models.DomainMutationResult{
			ConfigID: item.ConfigID,
			Hosts:    item.Hosts,
			Changed:  item.Changed,
			Added:    item.Added,
			Removed:  item.Removed,
		})
	}
	return &mutate
}
