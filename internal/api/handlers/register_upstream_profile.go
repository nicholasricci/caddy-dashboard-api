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

type registerUpstreamByProfileRequest struct {
	PrivateIP string `json:"private_ip" binding:"required"`
	DryRun    bool   `json:"dry_run"`
}

// RegisterUpstreamByProfile godoc
// @Summary Register upstream dials using an upstream profile
// @Description Adds upstream dials for all bindings in the profile on the first reachable Caddy node and propagates to peers. Authenticated via API key.
// @Tags upstream-profiles
// @Accept json
// @Produce json
// @Param id path string true "Upstream profile ID"
// @Param payload body registerUpstreamByProfileRequest true "Registration payload"
// @Success 200 {object} models.RegisterUpstreamProfileResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Failure 422 {object} models.ErrorResponse
// @Failure 502 {object} models.ErrorResponse
// @Security APIKeyAuth
// @Router /api/v1/upstream-profiles/{id}/register [post]
func (h *RegisterUpstreamHandler) RegisterUpstreamByProfile(c *gin.Context) {
	profileID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid upstream profile id"})
		return
	}
	validated, err := h.validatedAPIKey(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}
	profile, err := h.svc.GetProfile(c.Request.Context(), profileID)
	if err != nil {
		if errors.Is(err, services.ErrUpstreamProfileNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "upstream profile not found"})
			return
		}
		logRequestError(h.logger, c, "load upstream profile failed", err, zap.String("profile_id", profileID.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to load upstream profile"})
		return
	}
	if err := h.apiKeys.AuthorizeUpstreamProfile(validated, profileID, profile.DiscoveryConfigID, models.APIKeyScopeRegisterUpstream); err != nil {
		if errors.Is(err, services.ErrAPIKeyForbidden) || errors.Is(err, services.ErrAPIKeyProfileForbidden) || errors.Is(err, services.ErrAPIKeyScopeMissing) {
			c.JSON(http.StatusForbidden, models.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}
	var req registerUpstreamByProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	requestedBy := "api_key:" + validated.Name
	resp, err := h.svc.RegisterByProfile(c.Request.Context(), services.RegisterUpstreamByProfileInput{
		ProfileID:   profileID,
		PrivateIP:   req.PrivateIP,
		DryRun:      req.DryRun,
		RequestedBy: requestedBy,
	})
	if err != nil {
		if errors.Is(err, services.ErrUpstreamProfileNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "upstream profile not found"})
			return
		}
		if respondRegisterUpstreamError(c, err) {
			return
		}
		logRequestError(h.logger, c, "register upstream by profile failed", err, zap.String("profile_id", profileID.String()))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "register upstream by profile failed"})
		return
	}
	if h.audit != nil {
		targets := make([]gin.H, 0, len(resp.Targets))
		for _, t := range resp.Targets {
			targets = append(targets, gin.H{"config_id": t.ConfigID, "dial": t.Dial})
		}
		logAuditFailure(h.logger, c, "register_upstream_profile", "upstream_profile", profileID.String(), h.audit.Record(c.Request.Context(), requestedBy, "register_upstream_profile", "upstream_profile", profileID.String(), gin.H{
			"targets":        targets,
			"source_node_id": resp.SourceNodeID.String(),
			"changed":        resp.Changed,
		}))
	}
	c.JSON(http.StatusOK, toRegisterUpstreamProfileResponse(profileID, resp))
}

func toRegisterUpstreamProfileResponse(profileID uuid.UUID, in *services.RegisterUpstreamResult) models.RegisterUpstreamProfileResponse {
	out := models.RegisterUpstreamProfileResponse{
		UpstreamProfileID: profileID.String(),
		DiscoveryConfigID: in.DiscoveryConfigID.String(),
		SourceNodeID:      in.SourceNodeID.String(),
		Changed:           in.Changed,
		DryRun:            in.DryRun,
	}
	for _, t := range in.Targets {
		out.Targets = append(out.Targets, models.RegisterUpstreamProfileTarget{
			ConfigID: t.ConfigID,
			Dial:     t.Dial,
		})
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
