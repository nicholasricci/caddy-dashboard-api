package routes

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nicholasricci/caddy-dashboard/internal/api/handlers"
	"github.com/nicholasricci/caddy-dashboard/internal/api/middleware"
	"github.com/nicholasricci/caddy-dashboard/internal/auth"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type Dependencies struct {
	Logger                  *zap.Logger
	CORSAllowedOrigins      []string
	MaxBodyBytes            int64
	MaxApplyBodyBytes       int64
	EnableSwagger           bool
	AuthService             *auth.Service
	AuthHandler             *handlers.AuthHandler
	HealthHandler           *handlers.HealthHandler
	NodeHandler             *handlers.NodeHandler
	DiscoveryHandler        *handlers.DiscoveryHandler
	CaddyHandler            *handlers.CaddyHandler
	UserHandler             *handlers.UserHandler
	AuditHandler            *handlers.AuditHandler
	AdminHandler            *handlers.AdminHandler
	APIKeyHandler           *handlers.APIKeyHandler
	RegisterUpstreamHandler *handlers.RegisterUpstreamHandler
	RegisterDomainHandler   *handlers.RegisterDomainHandler
	UpstreamProfileHandler  *handlers.UpstreamProfileHandler
	DomainProfileHandler    *handlers.DomainProfileHandler
	APIKeyService           *services.APIKeyService
}

func NewRouter(dep Dependencies) *gin.Engine {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery(dep.Logger))
	r.Use(middleware.CORSMiddleware(dep.CORSAllowedOrigins))
	r.Use(middleware.MaxBodyBytes(dep.MaxBodyBytes))
	r.Use(middleware.RequestLogger(dep.Logger))
	if dep.EnableSwagger {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}

	api := r.Group("/api/v1")
	api.GET("/health", dep.HealthHandler.Health)
	api.GET("/ready", dep.HealthHandler.Ready)
	loginLimiter := middleware.NewLimiterStore(rate.Every(12*time.Second), 5)
	refreshLimiter := middleware.NewLimiterStore(rate.Every(6*time.Second), 10)
	api.POST("/auth/login", middleware.RateLimitByIP(loginLimiter), dep.AuthHandler.Login)
	api.POST("/auth/refresh", middleware.RateLimitByIP(refreshLimiter), dep.AuthHandler.Refresh)

	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware(dep.AuthService))
	protected.POST("/auth/logout", dep.AuthHandler.Logout)

	protected.GET("/nodes", dep.NodeHandler.List)
	protected.GET("/nodes/:id", dep.NodeHandler.Get)
	protected.GET("/discovery", dep.DiscoveryHandler.List)
	protected.GET("/discovery/:id", dep.DiscoveryHandler.Get)

	applyLimiter := middleware.NewLimiterStore(rate.Every(time.Second), 1)
	backfillLimiter := middleware.NewLimiterStore(rate.Every(30*time.Second), 1)
	admin := protected.Group("")
	admin.Use(middleware.RequireAdmin())
	admin.POST("/nodes", dep.NodeHandler.Create)
	admin.PUT("/nodes/:id", dep.NodeHandler.Update)
	admin.DELETE("/nodes/:id", dep.NodeHandler.Delete)

	admin.GET("/nodes/:id/config/live", dep.CaddyHandler.LiveConfig)
	admin.GET("/nodes/:id/config/live/ids", dep.CaddyHandler.ListConfigIDs)
	admin.GET("/nodes/:id/config/live/ids/:configId", dep.CaddyHandler.ConfigByID)
	admin.GET("/nodes/:id/config/live/ids/:configId/upstreams", dep.CaddyHandler.UpstreamsByID)
	admin.GET("/nodes/:id/config/live/ids/:configId/hosts", dep.CaddyHandler.HostsByID)
	admin.POST("/nodes/:id/config/mutate/domains", dep.CaddyHandler.MutateDomains)
	admin.POST("/nodes/:id/config/mutate/upstreams", dep.CaddyHandler.MutateUpstreams)
	admin.POST("/nodes/:id/config/propagate", dep.CaddyHandler.PropagateConfig)
	admin.POST("/nodes/:id/sync", dep.CaddyHandler.Sync)
	admin.POST("/nodes/:id/apply", middleware.MaxBodyBytes(dep.MaxApplyBodyBytes), middleware.RateLimitByIP(applyLimiter), dep.CaddyHandler.Apply)
	admin.POST("/nodes/:id/reload", dep.CaddyHandler.Reload)
	admin.GET("/nodes/:id/snapshots", dep.CaddyHandler.ListSnapshots)

	admin.POST("/discovery", dep.DiscoveryHandler.Create)
	admin.PUT("/discovery/:id", dep.DiscoveryHandler.Update)
	admin.DELETE("/discovery/:id", dep.DiscoveryHandler.Delete)
	admin.POST("/discovery/:id/run", dep.DiscoveryHandler.Run)
	admin.GET("/discovery/:id/snapshots", dep.DiscoveryHandler.ListSnapshots)

	admin.GET("/discovery/:id/upstream-profiles", dep.UpstreamProfileHandler.ListByDiscovery)
	admin.POST("/discovery/:id/upstream-profiles", dep.UpstreamProfileHandler.Create)
	admin.GET("/upstream-profiles/:id", dep.UpstreamProfileHandler.Get)
	admin.PUT("/upstream-profiles/:id", dep.UpstreamProfileHandler.Update)
	admin.DELETE("/upstream-profiles/:id", dep.UpstreamProfileHandler.Delete)

	admin.GET("/discovery/:id/domain-profiles", dep.DomainProfileHandler.ListByDiscovery)
	admin.POST("/discovery/:id/domain-profiles", dep.DomainProfileHandler.Create)
	admin.GET("/domain-profiles/:id", dep.DomainProfileHandler.Get)
	admin.PUT("/domain-profiles/:id", dep.DomainProfileHandler.Update)
	admin.DELETE("/domain-profiles/:id", dep.DomainProfileHandler.Delete)

	admin.GET("/users", dep.UserHandler.List)
	admin.GET("/users/:id", dep.UserHandler.Get)
	admin.POST("/users", dep.UserHandler.Create)
	admin.PUT("/users/:id", dep.UserHandler.Update)
	admin.DELETE("/users/:id", dep.UserHandler.Delete)
	admin.GET("/audit", dep.AuditHandler.List)
	admin.POST("/snapshots/backfill", middleware.RateLimitByIP(backfillLimiter), dep.AdminHandler.BackfillSnapshots)

	admin.GET("/api-keys", dep.APIKeyHandler.List)
	admin.POST("/api-keys", dep.APIKeyHandler.Create)
	admin.POST("/api-keys/:id/revoke", dep.APIKeyHandler.Revoke)
	admin.DELETE("/api-keys/:id", dep.APIKeyHandler.Delete)

	if dep.APIKeyService != nil && dep.RegisterUpstreamHandler != nil {
		registerLimiter := middleware.NewLimiterStore(rate.Every(time.Second), 20)
		m2m := api.Group("")
		m2m.Use(middleware.APIKeyAuthMiddleware(dep.APIKeyService))
		m2m.POST("/discovery/:id/register-upstream", middleware.RateLimitByIP(registerLimiter), dep.RegisterUpstreamHandler.RegisterUpstream)
		m2m.POST("/upstream-profiles/:id/register", middleware.RateLimitByIP(registerLimiter), dep.RegisterUpstreamHandler.RegisterUpstreamByProfile)
	}
	if dep.APIKeyService != nil && dep.RegisterDomainHandler != nil {
		registerLimiter := middleware.NewLimiterStore(rate.Every(time.Second), 20)
		m2m := api.Group("")
		m2m.Use(middleware.APIKeyAuthMiddleware(dep.APIKeyService))
		m2m.POST("/discovery/:id/register-domain", middleware.RateLimitByIP(registerLimiter), dep.RegisterDomainHandler.RegisterDomain)
		m2m.POST("/domain-profiles/:id/register", middleware.RateLimitByIP(registerLimiter), dep.RegisterDomainHandler.RegisterDomainByProfile)
	}

	return r
}
