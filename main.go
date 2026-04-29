package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bartek5186/procyon/controllers"
	"github.com/bartek5186/procyon/internal"
	"github.com/bartek5186/procyon/internal/apierr"
	"github.com/bartek5186/procyon/internal/authz"
	"github.com/bartek5186/procyon/internal/i18n"
	mid "github.com/bartek5186/procyon/internal/middleware"
	"github.com/bartek5186/procyon/internal/telemetry"
	"github.com/bartek5186/procyon/services"
	"github.com/bartek5186/procyon/store"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

const appVersion = "0.1.0"

var migrate bool
var configPath string

func init() {
	flag.BoolVar(&migrate, "migrate", false, "Run DB migrations on start")
	flag.StringVar(&configPath, "config", "", "Path to runtime config file")
}

func main() {
	flag.Parse()

	logger := internal.NewLogger()

	if err := i18n.LoadTranslations(); err != nil {
		logger.GetLogger().Fatal("failed to load translations", zap.Error(err))
	}

	if configPath == "" {
		configPath = "config/config.json"
	}

	config := internal.LoadConfiguration(configPath)
	obsConfig := config.Observability.WithDefaults(appVersion, config.AppName, config.Prod)
	logger = internal.NewLoggerWithConfig(
		config.Logging,
		zap.String("service", obsConfig.ServiceName),
		zap.String("env", obsConfig.Environment),
		zap.String("version", obsConfig.ServiceVersion),
	)

	db := internal.NewDatabaseConnection(config)
	casbinAuthz, err := authz.NewCasbinAuthorizer(db, config.RBAC.DefaultRole, config.RBAC.AdminIdentityIDs)
	if err != nil {
		logger.GetLogger().Fatal("failed to initialize casbin", zap.Error(err))
	}

	obs, err := telemetry.New(context.Background(), obsConfig, logger.GetLogger(), db)
	if err != nil {
		logger.GetLogger().Fatal("failed to initialize telemetry", zap.Error(err))
	}

	if migrate {
		if err := internal.MigrateRun(db, config); err != nil {
			logger.GetLogger().Fatal("failed to run migrations", zap.Error(err))
		}
	}

	appStore := store.NewAppStore(db, &config)
	appService := services.NewAppService(appStore, logger.GetLogger(), obs.BusinessMetrics())
	helloController := controllers.NewHelloController(appService, logger.GetLogger())

	var kratosAuth *mid.KratosAuth
	switch config.AuthProvider() {
	case "kratos":
		kratosAuth = mid.NewKratosAuth(config.AuthBaseURL())
	default:
		logger.GetLogger().Fatal("unsupported auth provider", zap.String("provider", config.AuthProvider()))
	}
	rbac := mid.NewCasbinRBAC(casbinAuthz)
	adminAuth := mid.NewAdminKeyAuth(config.Admin.SecretKey)

	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = apierr.Handler(logger.GetLogger())
	e.Validator = internal.NewInputValidator()
	e.Server.ReadHeaderTimeout = 5 * time.Second
	e.Server.ReadTimeout = 10 * time.Minute // albo 0, jeśli kontrolujesz to inaczej
	e.Server.WriteTimeout = 60 * time.Second
	e.Server.IdleTimeout = 120 * time.Second

	e.Use(middleware.RequestID())
	e.Use(mid.LanguageMiddleware("pl", config.Languages))
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     config.Domains,
		AllowMethods:     []string{echo.GET, echo.POST, echo.PUT, echo.PATCH, echo.DELETE, echo.OPTIONS},
		AllowCredentials: true,
	}))
	e.Use(obs.Middleware())

	e.Static("/", "static")

	e.GET(obsConfig.HealthPath, obs.HealthHandler)
	e.GET(obsConfig.ReadyPath, obs.ReadyHandler)
	e.GET(obsConfig.InfoPath, obs.InfoHandler)
	e.GET(obsConfig.MetricsPath, echo.WrapHandler(obs.MetricsHandler()))

	e.GET("/health", helloController.Health)
	e.GET("/hello", helloController.Hello)

	secured := e.Group("/v1", kratosAuth.RequireSession)
	secured.GET("/hello", helloController.HelloAuthenticated, rbac.Require("hello", "read"))
	securedAdmin := secured.Group("/admin", rbac.Require("hello", "manage"))
	securedAdmin.GET("/hello", helloController.HelloAdmin)

	admin := e.Group("/admin", adminAuth.RequireAdminKey)
	admin.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"status": "ok",
			"auth":   "admin_key",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	logger.GetLogger().Info(
		"server starting",
		zap.String("app", config.AppName),
		zap.String("address", config.ServerAddress()),
		zap.Bool("migrate", migrate),
		zap.String("metrics_path", obsConfig.MetricsPath),
	)

	serverErr := make(chan error, 1)
	go func() {
		if err := e.Start(config.ServerAddress()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case err := <-serverErr:
		logger.GetLogger().Fatal("server exited unexpectedly", zap.Error(err))
	case sig := <-signals:
		logger.GetLogger().Info("shutting down server", zap.String("signal", sig.String()))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.GetLogger().Error("server shutdown failed", zap.Error(err))
	}
	if err := obs.Shutdown(shutdownCtx); err != nil {
		logger.GetLogger().Error("telemetry shutdown failed", zap.Error(err))
	}
}
