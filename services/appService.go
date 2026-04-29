package services

import (
	"github.com/bartek5186/procyon/internal/telemetry"
	"github.com/bartek5186/procyon/store"
	"go.uber.org/zap"
)

type AppService struct {
	Store   store.Datastore
	Hello   *HelloService
	Metrics *telemetry.BusinessMetrics
	logger  *zap.Logger
}

func NewAppService(store store.Datastore, logger *zap.Logger, metrics *telemetry.BusinessMetrics) *AppService {
	return &AppService{
		Store:   store,
		Hello:   NewHelloService(store, logger, metrics),
		Metrics: metrics,
		logger:  logger,
	}
}
