package services

import (
	"context"
	"errors"
	"strings"

	"github.com/bartek5186/procyon/internal/i18n"
	"github.com/bartek5186/procyon/internal/middleware"
	"github.com/bartek5186/procyon/internal/telemetry"
	"github.com/bartek5186/procyon/models"
	"github.com/bartek5186/procyon/store"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type HelloService struct {
	Store   store.Datastore
	metrics *telemetry.BusinessMetrics
	logger  *zap.Logger
}

func NewHelloService(store store.Datastore, logger *zap.Logger, metrics *telemetry.BusinessMetrics) *HelloService {
	return &HelloService{
		Store:   store,
		metrics: metrics,
		logger:  logger,
	}
}

func (s *HelloService) Health(ctx context.Context) models.HealthResponse {
	lang := langFromContext(ctx)

	return models.HealthResponse{
		Status: i18n.T(lang, "health.status", nil),
		App:    s.Store.Config().AppName,
	}
}

func (s *HelloService) Greet(ctx context.Context, in models.HelloInput, identityID string) (*models.HelloResponse, error) {
	lang := langFromContext(ctx)

	message, source, err := s.resolveGreeting(ctx, lang)
	if err != nil {
		return nil, err
	}

	if name := strings.TrimSpace(in.Name); name != "" {
		message += " " + i18n.T(lang, "hello.named_suffix", map[string]string{"Name": name})
	}

	if identityID != "" {
		message += " " + i18n.T(lang, "hello.auth_suffix", map[string]string{"IdentityID": identityID})
	}

	s.metrics.Event(ctx, "hello_greeted",
		attribute.String("locale", lang),
		attribute.Bool("authenticated", identityID != ""),
		attribute.String("source", source),
	)

	return &models.HelloResponse{
		App:           s.Store.Config().AppName,
		Message:       message,
		Locale:        lang,
		Authenticated: identityID != "",
		IdentityID:    identityID,
		Source:        source,
	}, nil
}

func (s *HelloService) resolveGreeting(ctx context.Context, lang string) (string, string, error) {
	row, err := s.Store.Hello().GetBySlug(ctx, "hello")
	if err == nil {
		return row.Message, "db", nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", err
	}

	return i18n.T(lang, "hello.default", map[string]string{"App": s.Store.Config().AppName}), "i18n", nil
}

func langFromContext(ctx context.Context) string {
	if lang, ok := ctx.Value(middleware.KeyLang{}).(string); ok && lang != "" {
		return lang
	}
	return "pl"
}
