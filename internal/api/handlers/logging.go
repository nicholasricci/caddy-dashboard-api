package handlers

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func nopLogger(logger *zap.Logger) *zap.Logger {
	if logger != nil {
		return logger
	}
	return zap.NewNop()
}

func requestLogFields(c *gin.Context, extra ...zap.Field) []zap.Field {
	fields := make([]zap.Field, 0, len(extra)+4)
	if c != nil {
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
	}
	fields = append(fields, extra...)
	return fields
}

func logRequestError(logger *zap.Logger, c *gin.Context, message string, err error, extra ...zap.Field) {
	if err == nil {
		return
	}
	fields := append([]zap.Field{}, extra...)
	fields = append(fields, zap.Error(err))
	nopLogger(logger).Error(message, requestLogFields(c, fields...)...)
}

func logAuditFailure(logger *zap.Logger, c *gin.Context, action, resource, resourceID string, err error) {
	if err == nil {
		return
	}
	nopLogger(logger).Warn("audit record failed", requestLogFields(c,
		zap.String("action", action),
		zap.String("resource", resource),
		zap.String("resource_id", resourceID),
		zap.Error(err),
	)...)
}
