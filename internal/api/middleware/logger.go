package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RequestID ensures X-Request-ID is set (echoed or generated) for log correlation.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if rid == "" {
			rid = uuid.New().String()
		}
		c.Writer.Header().Set("X-Request-ID", rid)
		c.Set("request_id", rid)
		c.Next()
	}
}

func RequestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		fields := []zap.Field{
			zap.String("request_id", c.GetString("request_id")),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		}
		if u := c.GetString("username"); u != "" {
			fields = append(fields, zap.String("username", u))
		}
		log.Info("http_request", fields...)
	}
}
