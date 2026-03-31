package services

import (
	"github.com/bartek5186/procyon/store"
	"github.com/sirupsen/logrus"
)

type AppService struct {
	Store  store.Datastore
	Hello  *HelloService
	logger *logrus.Logger
}

func NewAppService(store store.Datastore, logger *logrus.Logger) *AppService {
	return &AppService{
		Store:  store,
		Hello:  NewHelloService(store, logger),
		logger: logger,
	}
}
