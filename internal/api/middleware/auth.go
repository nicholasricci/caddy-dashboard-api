package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nicholasricci/caddy-dashboard/internal/auth"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

func AuthMiddleware(authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "missing bearer token"})
			return
		}
		token := header
		if strings.HasPrefix(strings.ToLower(header), "bearer ") {
			token = strings.TrimSpace(header[7:])
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "missing bearer token"})
			return
		}
		claims, err := authSvc.ValidateToken(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid token"})
			return
		}
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}
