package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
	"go.uber.org/zap"
)

type AuditHandler struct {
	svc    *services.AuditService
	logger *zap.Logger
}

func NewAuditHandler(svc *services.AuditService, logger *zap.Logger) *AuditHandler {
	return &AuditHandler{svc: svc, logger: nopLogger(logger)}
}

// List godoc
// @Summary List audit logs
// @Tags audit
// @Produce json
// @Param action query string false "Filter by action (see GET /audit/types)"
// @Param resource query string false "Filter by resource (see GET /audit/types)"
// @Param actor query string false "Filter by actor (exact match)"
// @Param resource_id query string false "Filter by resource ID (exact match)"
// @Param from query string false "Include logs from this time (RFC3339)"
// @Param to query string false "Include logs up to this time (RFC3339)"
// @Param limit query int false "Page size (default 20, max 100)"
// @Param offset query int false "Page offset (default 0)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/audit [get]
func (h *AuditHandler) List(c *gin.Context) {
	filter, err := parseAuditListFilter(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	limit, offset := parseLimitOffset(c)
	items, total, err := h.svc.ListPaginated(c.Request.Context(), filter, limit, offset)
	if err != nil {
		logRequestError(h.logger, c, "list audit logs failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list audit logs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "meta": models.PaginationMeta{Total: total, Limit: limit, Offset: offset}})
}

// ListTypes godoc
// @Summary List allowed audit filter values
// @Tags audit
// @Produce json
// @Success 200 {object} models.AuditTypesResponse
// @Security BearerAuth
// @Router /api/v1/audit/types [get]
func (h *AuditHandler) ListTypes(c *gin.Context) {
	c.JSON(http.StatusOK, models.AuditTypesResponse{
		Actions:   models.AuditActions(),
		Resources: models.AuditResources(),
	})
}

func parseAuditListFilter(c *gin.Context) (models.AuditListFilter, error) {
	var filter models.AuditListFilter

	if v := strings.TrimSpace(c.Query("action")); v != "" {
		if !models.IsValidAuditAction(v) {
			return filter, errors.New("invalid action")
		}
		filter.Action = v
	}
	if v := strings.TrimSpace(c.Query("resource")); v != "" {
		if !models.IsValidAuditResource(v) {
			return filter, errors.New("invalid resource")
		}
		filter.Resource = v
	}
	if v := strings.TrimSpace(c.Query("actor")); v != "" {
		filter.Actor = v
	}
	if v := strings.TrimSpace(c.Query("resource_id")); v != "" {
		filter.ResourceID = v
	}

	fromRaw := strings.TrimSpace(c.Query("from"))
	toRaw := strings.TrimSpace(c.Query("to"))
	if fromRaw != "" {
		from, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			return filter, errors.New("invalid from: expected RFC3339 timestamp")
		}
		filter.From = &from
	}
	if toRaw != "" {
		to, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			return filter, errors.New("invalid to: expected RFC3339 timestamp")
		}
		filter.To = &to
	}
	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return filter, errors.New("from must be before or equal to to")
	}

	return filter, nil
}
