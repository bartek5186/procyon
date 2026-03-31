package store

import (
	"context"
	"errors"

	"github.com/bartek5186/procyon/internal/middleware"
	"github.com/bartek5186/procyon/models"
	"gorm.io/gorm"
)

type HelloStore struct {
	db *gorm.DB
}

func NewHelloStore(db *gorm.DB) *HelloStore {
	return &HelloStore{db: db}
}

func (s *HelloStore) GetBySlug(ctx context.Context, slug string) (*models.HelloMessage, error) {
	cands, _ := ctx.Value(middleware.KeyLangCandidates{}).([]string)
	if len(cands) == 0 {
		if lang, ok := ctx.Value(middleware.KeyLang{}).(string); ok && lang != "" {
			cands = []string{lang}
		}
	}
	if len(cands) == 0 {
		cands = []string{"pl"}
	}

	var rows []models.HelloMessage
	if err := s.db.WithContext(ctx).
		Where("slug = ? AND lang IN ?", slug, cands).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	for _, lang := range cands {
		for i := range rows {
			if rows[i].Lang == lang {
				return &rows[i], nil
			}
		}
	}

	return nil, errors.New("hello message resolution failed")
}
