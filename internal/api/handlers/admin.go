package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
	"go.uber.org/zap"
)

type AdminHandler struct {
	snapshots *repository.SnapshotRepository
	audit     *services.AuditService
	logger    *zap.Logger
}

func NewAdminHandler(snapshots *repository.SnapshotRepository, audit *services.AuditService, logger *zap.Logger) *AdminHandler {
	return &AdminHandler{
		snapshots: snapshots,
		audit:     audit,
		logger:    nopLogger(logger),
	}
}

// BackfillSnapshots godoc
// @Summary Re-run snapshot discovery backfill
// @Description Reassigns discovery_config_id on legacy node-scoped snapshots for discovery configs using snapshot_scope=group.
// @Description Idempotent and safe to re-run. Endpoint is admin-only and rate-limited.
// @Tags admin
// @Produce json
// @Success 200 {object} models.BackfillSnapshotsResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/snapshots/backfill [post]
func (h *AdminHandler) BackfillSnapshots(c *gin.Context) {
	start := time.Now()
	rows, err := h.snapshots.BackfillDiscoveryConfigIDs(c.Request.Context())
	duration := time.Since(start)
	if err != nil {
		logRequestError(h.logger, c, "snapshot backfill failed", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "backfill failed"})
		return
	}

	logAuditFailure(h.logger, c, "backfill", "snapshot", "", h.audit.Record(
		c.Request.Context(),
		c.GetString("username"),
		"backfill",
		"snapshot",
		"",
		gin.H{
			"rows_updated": rows,
			"duration_ms":  duration.Milliseconds(),
		},
	))

	c.JSON(http.StatusOK, models.BackfillSnapshotsResponse{
		RowsUpdated: rows,
		DurationMs:  duration.Milliseconds(),
	})
}
