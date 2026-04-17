package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/nicholasricci/caddy-dashboard/docs"
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
	autoMigrate := flag.Bool("auto-migrate", false, "run automigrate at startup")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		fallback, _ := logger.New("info")
		fallback.Fatal("config load failed", zap.Error(err))
	}

	gin.SetMode(cfg.Server.GinMode)

	log, err := logger.New(cfg.Observability.LogLevel)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	db, err := database.NewConnection(ctx, cfg.Database)
	if err != nil {
		log.Fatal("database connection failed", zap.Error(err))
	}
	defer closeDB(log, db)

	if *autoMigrate {
		if err := database.AutoMigrate(db); err != nil {
			log.Fatal("database migration failed", zap.Error(err))
		}
	}

	awsClients, err := awssvc.NewMultiRegionClient(ctx, cfg.AWS.Profile, cfg.AWS.Regions)
	if err != nil {
		log.Fatal("aws client init failed", zap.Error(err))
	}

	nodeRepo := repository.NewNodeRepository(db)
	discoveryRepo := repository.NewDiscoveryRepository(db)
	snapshotRepo := repository.NewSnapshotRepository(db)
	userRepo := repository.NewUserRepository(db)

	refreshRepo := repository.NewRefreshTokenRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	authSvc := auth.NewService(cfg.Auth, userRepo, refreshRepo)
	ec2Svc := awssvc.NewEC2Service(awsClients)
	ssmSvc := awssvc.NewSSMService(awsClients)
	executor := caddysvc.NewSSMExecutor(ssmSvc)
	internalCaddySvc := caddysvc.NewService(nodeRepo, snapshotRepo, executor)

	nodeSvc := services.NewNodeService(nodeRepo)
	discoverySvc := services.NewDiscoveryService(discoveryRepo, nodeRepo, ec2Svc, ssmSvc)
	caddySvc := services.NewCaddyService(internalCaddySvc, snapshotRepo)
	userSvc := services.NewUserService(userRepo)
	auditSvc := services.NewAuditService(auditRepo)

	authHandler := handlers.NewAuthHandler(authSvc)
	healthHandler := handlers.NewHealthHandler(
		func(ctx context.Context) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.PingContext(ctx)
		},
		func(context.Context) error {
			if len(cfg.AWS.Regions) == 0 {
				return errors.New("no aws regions configured")
			}
			return nil
		},
	)
	nodeHandler := handlers.NewNodeHandler(nodeSvc, auditSvc)
	discoveryHandler := handlers.NewDiscoveryHandler(discoverySvc, auditSvc)
	caddyHandler := handlers.NewCaddyHandler(caddySvc, auditSvc)
	userHandler := handlers.NewUserHandler(userSvc, auditSvc)
	auditHandler := handlers.NewAuditHandler(auditSvc)

	router := routes.NewRouter(routes.Dependencies{
		Logger:             log,
		CORSAllowedOrigins: cfg.Server.CORSAllowedOrigins,
		MaxBodyBytes:       cfg.Server.MaxBodyBytes,
		MaxApplyBodyBytes:  cfg.Server.MaxApplyBodyBytes,
		EnableSwagger:      cfg.Server.EnableSwagger && cfg.Server.GinMode != "release",
		AuthService:        authSvc,
		AuthHandler:        authHandler,
		HealthHandler:      healthHandler,
		NodeHandler:        nodeHandler,
		DiscoveryHandler:   discoveryHandler,
		CaddyHandler:       caddyHandler,
		UserHandler:        userHandler,
		AuditHandler:       auditHandler,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           router,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		log.Info("server started", zap.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server failed", zap.Error(err))
		}
	}()

	waitForShutdown(log, srv, cfg.Server.ShutdownTimeout)
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

func waitForShutdown(log *zap.Logger, srv *http.Server, timeout time.Duration) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	}
	log.Info("server stopped")
}
