package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"go.uber.org/zap"
)

func Recovery(logger *zap.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = zap.NewNop()
	}

	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		fields := []zap.Field{
			zap.Any("panic", recovered),
			zap.ByteString("stack", debug.Stack()),
		}
		if rid := c.GetString("request_id"); rid != "" {
			fields = append(fields, zap.String("request_id", rid))
		}
		if c.Request != nil {
			fields = append(fields,
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
			)
		}
		if username := c.GetString("username"); username != "" {
			fields = append(fields, zap.String("username", username))
		}

		logger.Error("panic recovered", fields...)
		c.AbortWithStatusJSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
	})
}
