package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("role") != models.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, models.ErrorResponse{Error: "admin role required"})
			return
		}
		c.Next()
	}
}
