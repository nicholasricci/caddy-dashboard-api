package handlers

import (
	"net/http"

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
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/audit [get]
func (h *AuditHandler) List(c *gin.Context) {
	limit, offset := parseLimitOffset(c)
	items, total, err := h.svc.ListPaginated(c.Request.Context(), limit, offset)
	if err != nil {
		logRequestError(h.logger, c, "list audit logs failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list audit logs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "meta": models.PaginationMeta{Total: total, Limit: limit, Offset: offset}})
}
