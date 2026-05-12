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
	"github.com/google/uuid"
	_ "github.com/nicholasricci/caddy-dashboard/docs"
	"github.com/nicholasricci/caddy-dashboard/internal/api/handlers"
	"github.com/nicholasricci/caddy-dashboard/internal/api/routes"
	"github.com/nicholasricci/caddy-dashboard/internal/auth"
	awssvc "github.com/nicholasricci/caddy-dashboard/internal/aws"
	caddysvc "github.com/nicholasricci/caddy-dashboard/internal/caddy"
	"github.com/nicholasricci/caddy-dashboard/internal/cloud/azure"
	gcpcloud "github.com/nicholasricci/caddy-dashboard/internal/cloud/gcp"
	"github.com/nicholasricci/caddy-dashboard/internal/config"
	"github.com/nicholasricci/caddy-dashboard/internal/database"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"github.com/nicholasricci/caddy-dashboard/internal/secrets"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
	"github.com/nicholasricci/caddy-dashboard/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// @title Caddy Dashboard API
// @version 1.0
// @description API for managing Caddy nodes: AWS SSM, SSH, or direct HTTP admin; discovery includes AWS, static IP, GCP labels, and Azure tags.
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
	if err := database.BackfillCaddyNodes(db); err != nil {
		log.Warn("node transport backfill failed", zap.Error(err))
	}

	var awsClients *awssvc.MultiRegionClient
	if len(cfg.AWS.Regions) > 0 {
		c, err := awssvc.NewMultiRegionClient(ctx, cfg.AWS.Profile, cfg.AWS.Regions)
		if err != nil {
			log.Fatal("aws client init failed", zap.Error(err))
		}
		awsClients = c
	} else if !cfg.AWS.Optional {
		log.Fatal("aws.regions is empty; set regions in config or aws.optional=true")
	} else {
		log.Info("starting without AWS (no regions configured, aws.optional=true)")
	}

	nodeRepo := repository.NewNodeRepository(db)
	discoveryRepo := repository.NewDiscoveryRepository(db)
	snapshotRepo := repository.NewSnapshotRepository(db)
	userRepo := repository.NewUserRepository(db)

	if rows, err := snapshotRepo.BackfillDiscoveryConfigIDs(ctx); err != nil {
		log.Warn("snapshot discovery group backfill failed", zap.Error(err))
	} else if rows > 0 {
		log.Info("snapshot discovery group backfill applied", zap.Int64("rows", rows))
	}

	refreshRepo := repository.NewRefreshTokenRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	authSvc := auth.NewService(cfg.Auth, userRepo, refreshRepo)

	var ec2Svc *awssvc.EC2Service
	var ssmSvc *awssvc.SSMService
	var secretsSvc *awssvc.SecretsService
	if awsClients != nil {
		ec2Svc = awssvc.NewEC2Service(awsClients)
		ssmSvc = awssvc.NewSSMService(awsClients)
		secretsSvc = awssvc.NewSecretsService(awsClients)
	}

	secretResolver := secrets.NewResolver(secretsSvc, cfg.Caddy.SecretCacheTTL)
	sshPool := caddysvc.NewSSHPool(cfg.Caddy.SSHIdleTTL)

	var ssmExec caddysvc.RemoteExecutor
	if ssmSvc != nil {
		ssmExec = caddysvc.NewSSMExecutor(ssmSvc)
	} else {
		ssmExec = &caddysvc.ErrRemoteExecutor{Err: caddysvc.ErrTransportNotConfigured}
	}
	httpExec := caddysvc.NewHTTPAdminExecutor(secretResolver, cfg.Caddy.HTTPAdminTimeout, cfg.Caddy.HTTPMaxResponseBody)
	sshExec := caddysvc.NewSSHExecutor(sshPool, secretResolver, cfg.Caddy.SSHTimeout, cfg.Caddy.SSHTimeout)
	dispatcher := caddysvc.NewDispatcher(map[string]caddysvc.RemoteExecutor{
		models.TransportAWSSSM:    ssmExec,
		models.TransportHTTPAdmin: httpExec,
		models.TransportSSH:       sshExec,
	})
	internalCaddySvc := caddysvc.NewService(
		nodeRepo,
		discoveryRepo,
		snapshotRepo,
		dispatcher,
		caddysvc.WithCache(caddysvc.NewInMemoryConfigCacheStore()),
		caddysvc.WithCacheTTL(cfg.Caddy.CacheTTL),
	)
	nodeRepo.OnNodeDeleted = func(_ context.Context, id uuid.UUID) {
		sshPool.Evict(id.String())
		internalCaddySvc.PurgeNodeState(id)
	}

	nodeSvc := services.NewNodeService(nodeRepo)
	discoverySvc := services.NewDiscoveryService(discoveryRepo, nodeRepo, services.DiscoveryDeps{
		EC2:       ec2Svc,
		SSM:       ssmSvc,
		GCPLabels: gcpcloud.NewRunner(),
		AzureTags: azure.NewRunner(),
	})
	caddySvc := services.NewCaddyService(internalCaddySvc, nodeRepo, discoveryRepo, snapshotRepo)
	userSvc := services.NewUserService(userRepo)
	auditSvc := services.NewAuditService(auditRepo)

	authHandler := handlers.NewAuthHandler(authSvc, log)
	healthHandler := handlers.NewHealthHandler(
		func(ctx context.Context) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.PingContext(ctx)
		},
	)
	nodeHandler := handlers.NewNodeHandler(nodeSvc, auditSvc, log)
	discoveryHandler := handlers.NewDiscoveryHandler(discoverySvc, caddySvc, auditSvc, log)
	caddyHandler := handlers.NewCaddyHandler(caddySvc, auditSvc, log)
	userHandler := handlers.NewUserHandler(userSvc, auditSvc, log)
	auditHandler := handlers.NewAuditHandler(auditSvc, log)
	adminHandler := handlers.NewAdminHandler(snapshotRepo, auditSvc, log)

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
		AdminHandler:       adminHandler,
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
