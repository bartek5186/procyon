package store

import (
	coreconfig "github.com/bartek5186/procyon-core/config"
	"gorm.io/gorm"
)

type Datastore interface {
	Db() *gorm.DB
	Config() *coreconfig.Config
	// procyon:module-store-interface
	Hello() *HelloStore
}

type AppStore struct {
	db     *gorm.DB
	config *coreconfig.Config
	// procyon:module-store-fields
	hello *HelloStore
}

func NewAppStore(db *gorm.DB, cfg *coreconfig.Config) *AppStore {
	return &AppStore{
		db:     db,
		config: cfg,
		// procyon:module-store-init
		hello: NewHelloStore(db),
	}
}

func (s *AppStore) Db() *gorm.DB {
	return s.db
}

func (s *AppStore) Config() *coreconfig.Config {
	return s.config
}

func (s *AppStore) Hello() *HelloStore {
	return s.hello
}

// procyon:module-store-methods
