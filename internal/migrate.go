package internal

import (
	"context"
	"fmt"

	"github.com/bartek5186/procyon/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func MigrateRun(db *gorm.DB) error {
	if err := db.Transaction(func(tx *gorm.DB) error {
		tx = tx.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci")

		if err := tx.AutoMigrate(
			&models.HelloMessage{},
		); err != nil {
			return err
		}

		return seedHelloMessages(context.Background(), tx)
	}); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return nil
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
