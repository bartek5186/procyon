package internal

import (
	"context"

	"github.com/bartek5186/procyon/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func init() {
	RegisterSeeder(seedHelloMessages)
}

func seedHelloMessages(ctx context.Context, db *gorm.DB) error {
	items := []models.HelloMessage{
		{Slug: "hello", Lang: "pl", Message: "Witaj z bazy danych Procyon."},
		{Slug: "hello", Lang: "en", Message: "Hello from the Procyon database."},
	}

	return db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "slug"}, {Name: "lang"}},
			DoNothing: true,
		}).
		Create(&items).Error
}
