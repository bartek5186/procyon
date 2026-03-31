package store

import (
	"github.com/bartek5186/procyon/internal"
	"gorm.io/gorm"
)

type Datastore interface {
	Db() *gorm.DB
	Config() *internal.Config
	Hello() *HelloStore
}

type AppStore struct {
	db     *gorm.DB
	config *internal.Config
	hello  *HelloStore
}

func NewAppStore(db *gorm.DB, cfg *internal.Config) *AppStore {
	return &AppStore{
		db:     db,
		config: cfg,
		hello:  NewHelloStore(db),
	}
}

func (s *AppStore) Db() *gorm.DB {
	return s.db
}

func (s *AppStore) Config() *internal.Config {
	return s.config
}

func (s *AppStore) Hello() *HelloStore {
	return s.hello
}
