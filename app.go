package main

import (
	"context"
	"errors"
	"fmt"
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

type application struct {
	obs        *telemetry.Manager
	kratosAuth *mid.KratosAuth
	rbac       *mid.CasbinRBAC
	adminAuth  *mid.AdminKeyAuth
	hello      *controllers.HelloController
}

func run() error {
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

	var kratosAuth *mid.KratosAuth
	switch config.AuthProvider() {
	case "kratos":
		kratosAuth = mid.NewKratosAuth(config.AuthBaseURL())
	default:
		logger.GetLogger().Fatal("unsupported auth provider", zap.String("provider", config.AuthProvider()))
	}

	app := &application{
		obs:        obs,
		kratosAuth: kratosAuth,
		rbac:       mid.NewCasbinRBAC(casbinAuthz),
		adminAuth:  mid.NewAdminKeyAuth(config.Admin.SecretKey),
		hello:      controllers.NewHelloController(appService, logger.GetLogger()),
	}

	public := newPublicServer(config, obs)
	registerPublicRoutes(public, app)

	admin := newAdminServer()
	registerAdminRoutes(admin, app)

	upload := newUploadServer()
	registerUploadRoutes(upload, app)

	logger.GetLogger().Info(
		"server starting",
		zap.String("app", config.AppName),
		zap.String("public", config.PublicAddress()),
		zap.String("admin", config.AdminAddress()),
		zap.String("upload", config.UploadAddress()),
		zap.Bool("migrate", migrate),
	)

	type serverErr struct {
		name string
		err  error
	}
	errs := make(chan serverErr, 3)
	startServer := func(name, addr string, e *echo.Echo) {
		go func() {
			if err := e.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errs <- serverErr{name, err}
			}
		}()
	}
	startServer("public", config.PublicAddress(), public)
	startServer("admin", config.AdminAddress(), admin)
	startServer("upload", config.UploadAddress(), upload)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case se := <-errs:
		return fmt.Errorf("server %s: %w", se.name, se.err)
	case sig := <-signals:
		logger.GetLogger().Info("shutting down", zap.String("signal", sig.String()))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for name, e := range map[string]*echo.Echo{"public": public, "admin": admin, "upload": upload} {
		if err := e.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.GetLogger().Error("shutdown failed", zap.String("server", name), zap.Error(err))
		}
	}
	if err := obs.Shutdown(shutdownCtx); err != nil {
		logger.GetLogger().Error("telemetry shutdown failed", zap.Error(err))
	}

	return nil
}

func newPublicServer(config internal.Config, obs *telemetry.Manager) *echo.Echo {
	e := newBaseServer()
	e.Server.ReadTimeout = 15 * time.Second
	e.Server.WriteTimeout = 30 * time.Second
	e.Server.IdleTimeout = 60 * time.Second

	e.Use(mid.LanguageMiddleware("pl", config.Languages))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     config.Domains,
		AllowMethods:     []string{echo.GET, echo.POST, echo.PUT, echo.PATCH, echo.DELETE, echo.OPTIONS},
		AllowCredentials: true,
	}))
	e.Use(obs.Middleware())

	return e
}

func newAdminServer() *echo.Echo {
	e := newBaseServer()
	e.Server.ReadTimeout = 30 * time.Second
	e.Server.WriteTimeout = 30 * time.Second
	e.Server.IdleTimeout = 30 * time.Second

	return e
}

func newUploadServer() *echo.Echo {
	e := newBaseServer()
	e.Server.ReadTimeout = 10 * time.Minute
	e.Server.WriteTimeout = 10 * time.Minute
	e.Server.IdleTimeout = 120 * time.Second

	return e
}

func newBaseServer() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = apierr.Handler(internal.NewLogger().GetLogger())
	e.Validator = internal.NewInputValidator()
	e.Server.ReadHeaderTimeout = 5 * time.Second

	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())

	return e
}
