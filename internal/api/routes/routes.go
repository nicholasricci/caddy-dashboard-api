package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/nicholasricci/caddy-dashboard/internal/api/handlers"
	"github.com/nicholasricci/caddy-dashboard/internal/api/middleware"
	"github.com/nicholasricci/caddy-dashboard/internal/auth"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

type Dependencies struct {
	Logger             *zap.Logger
	CORSAllowedOrigins []string
	AuthService        *auth.Service
	AuthHandler        *handlers.AuthHandler
	HealthHandler      *handlers.HealthHandler
	NodeHandler        *handlers.NodeHandler
	DiscoveryHandler   *handlers.DiscoveryHandler
	CaddyHandler       *handlers.CaddyHandler
	UserHandler        *handlers.UserHandler
}

func NewRouter(dep Dependencies) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware(dep.CORSAllowedOrigins))
	r.Use(middleware.RequestID())
	r.Use(middleware.RequestLogger(dep.Logger))
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	api := r.Group("/api/v1")
	api.GET("/health", dep.HealthHandler.Health)
	api.POST("/auth/login", dep.AuthHandler.Login)
	api.POST("/auth/refresh", dep.AuthHandler.Refresh)

	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware(dep.AuthService))

	protected.GET("/nodes", dep.NodeHandler.List)
	protected.GET("/nodes/:id", dep.NodeHandler.Get)
	protected.GET("/discovery", dep.DiscoveryHandler.List)
	protected.GET("/discovery/:id", dep.DiscoveryHandler.Get)

	admin := protected.Group("")
	admin.Use(middleware.RequireAdmin())
	admin.POST("/nodes", dep.NodeHandler.Create)
	admin.PUT("/nodes/:id", dep.NodeHandler.Update)
	admin.DELETE("/nodes/:id", dep.NodeHandler.Delete)

	admin.GET("/nodes/:id/config/live", dep.CaddyHandler.LiveConfig)
	admin.POST("/nodes/:id/sync", dep.CaddyHandler.Sync)
	admin.POST("/nodes/:id/apply", dep.CaddyHandler.Apply)
	admin.POST("/nodes/:id/reload", dep.CaddyHandler.Reload)
	protected.GET("/nodes/:id/snapshots", dep.CaddyHandler.ListSnapshots)

	admin.POST("/discovery", dep.DiscoveryHandler.Create)
	admin.PUT("/discovery/:id", dep.DiscoveryHandler.Update)
	admin.DELETE("/discovery/:id", dep.DiscoveryHandler.Delete)
	admin.POST("/discovery/:id/run", dep.DiscoveryHandler.Run)

	admin.GET("/users", dep.UserHandler.List)
	admin.GET("/users/:id", dep.UserHandler.Get)
	admin.POST("/users", dep.UserHandler.Create)
	admin.PUT("/users/:id", dep.UserHandler.Update)
	admin.DELETE("/users/:id", dep.UserHandler.Delete)

	return r
}
