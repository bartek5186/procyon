package services

import (
	"github.com/bartek5186/procyon/store"
	"go.uber.org/zap"
)

type AppService struct {
	Store  store.Datastore
	Hello  *HelloService
	logger *zap.Logger
}

func NewAppService(store store.Datastore, logger *zap.Logger) *AppService {
	return &AppService{
		Store:  store,
		Hello:  NewHelloService(store, logger),
		logger: logger,
	}
}
