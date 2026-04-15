package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nicholasricci/caddy-dashboard/internal/api/handlers"
	"github.com/nicholasricci/caddy-dashboard/internal/api/routes"
	"github.com/nicholasricci/caddy-dashboard/internal/auth"
	awssvc "github.com/nicholasricci/caddy-dashboard/internal/aws"
	caddysvc "github.com/nicholasricci/caddy-dashboard/internal/caddy"
	"github.com/nicholasricci/caddy-dashboard/internal/config"
	"github.com/nicholasricci/caddy-dashboard/internal/database"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
	"github.com/nicholasricci/caddy-dashboard/pkg/logger"
	_ "github.com/nicholasricci/caddy-dashboard/docs"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// @title Caddy Dashboard API
// @version 1.0
// @description API for managing Caddy nodes on AWS via SSM.
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	gin.SetMode(cfg.Server.GinMode)

	log, err := logger.New(cfg.Observability.LogLevel)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatal("database connection failed", zap.Error(err))
	}
	defer closeDB(log, db)

	if err := database.AutoMigrate(db); err != nil {
		log.Fatal("database migration failed", zap.Error(err))
	}

	awsClients, err := awssvc.NewMultiRegionClient(ctx, cfg.AWS.Profile, cfg.AWS.Regions)
	if err != nil {
		log.Fatal("aws client init failed", zap.Error(err))
	}

	nodeRepo := repository.NewNodeRepository(db)
	discoveryRepo := repository.NewDiscoveryRepository(db)
	snapshotRepo := repository.NewSnapshotRepository(db)
	userRepo := repository.NewUserRepository(db)

	authSvc := auth.NewService(cfg.Auth, userRepo)
	ec2Svc := awssvc.NewEC2Service(awsClients)
	ssmSvc := awssvc.NewSSMService(awsClients)
	executor := caddysvc.NewSSMExecutor(ssmSvc)
	internalCaddySvc := caddysvc.NewService(nodeRepo, snapshotRepo, executor)

	nodeSvc := services.NewNodeService(nodeRepo)
	discoverySvc := services.NewDiscoveryService(discoveryRepo, nodeRepo, ec2Svc, ssmSvc)
	caddySvc := services.NewCaddyService(internalCaddySvc, snapshotRepo)
	userSvc := services.NewUserService(userRepo)

	authHandler := handlers.NewAuthHandler(authSvc)
	healthHandler := handlers.NewHealthHandler()
	nodeHandler := handlers.NewNodeHandler(nodeSvc)
	discoveryHandler := handlers.NewDiscoveryHandler(discoverySvc)
	caddyHandler := handlers.NewCaddyHandler(caddySvc)
	userHandler := handlers.NewUserHandler(userSvc)

	router := routes.NewRouter(routes.Dependencies{
		Logger:             log,
		CORSAllowedOrigins: cfg.Server.CORSAllowedOrigins,
		AuthService:        authSvc,
		AuthHandler:        authHandler,
		HealthHandler:      healthHandler,
		NodeHandler:        nodeHandler,
		DiscoveryHandler:   discoveryHandler,
		CaddyHandler:       caddyHandler,
		UserHandler:        userHandler,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: router,
	}

	go func() {
		log.Info("server started", zap.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server failed", zap.Error(err))
		}
	}()

	waitForShutdown(log, srv)
}

func closeDB(log *zap.Logger, db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Warn("database sql handle unavailable", zap.Error(err))
		return
	}
	if err := sqlDB.Close(); err != nil {
		log.Warn("database close", zap.Error(err))
	}
}

func waitForShutdown(log *zap.Logger, srv *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	}
	log.Info("server stopped")
}
