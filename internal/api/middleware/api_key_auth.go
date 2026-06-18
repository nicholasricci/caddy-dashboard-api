package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
)

// APIKeyAuthMiddleware authenticates machine requests using Bearer API keys (cdk_live_…).
func APIKeyAuthMiddleware(apiKeys *services.APIKeyService) gin.HandlerFunc {
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
		validated, err := apiKeys.Validate(c.Request.Context(), token)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrAPIKeyRevoked):
				c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "api key revoked"})
			case errors.Is(err, services.ErrAPIKeyExpired):
				c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "api key expired"})
			default:
				c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid api key"})
			}
			return
		}
		c.Set("api_key_id", validated.ID.String())
		c.Set("api_key_name", validated.Name)
		c.Set("api_key_scopes", validated.Scopes)
		c.Set("api_key_allowed_discovery_ids", validated.AllowedDiscoveryConfigIDs)
		c.Set("api_key_allowed_upstream_profile_ids", validated.AllowedUpstreamProfileIDs)
		c.Set("auth_type", "api_key")
		c.Next()
	}
}
