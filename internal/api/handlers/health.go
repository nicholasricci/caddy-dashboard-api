package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	readyChecks []func(context.Context) error
}

func NewHealthHandler(readyChecks ...func(context.Context) error) *HealthHandler {
	return &HealthHandler{readyChecks: readyChecks}
}

// Health godoc
// @Summary Health check
// @Description Returns service health status
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /api/v1/health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Ready godoc
// @Summary Readiness check
// @Description Returns readiness status after running dependency checks
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 503 {object} models.ErrorResponse
// @Router /api/v1/ready [get]
func (h *HealthHandler) Ready(c *gin.Context) {
	for _, check := range h.readyChecks {
		if err := check(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
