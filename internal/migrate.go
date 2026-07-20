package internal

import (
	"context"
	"embed"

	coreconfig "github.com/bartek5186/procyon-core/config"
	coremigrations "github.com/bartek5186/procyon-core/migrations"
	"github.com/bartek5186/procyon/models"
	"gorm.io/gorm"
)

//go:embed migrations/mysql/* migrations/postgres/*
var migrationFiles embed.FS

func MigrateRun(ctx context.Context, db *gorm.DB, cfg coreconfig.Config) error {
	return coremigrations.Run(ctx, db, cfg, coremigrations.Plan{
		SQLFiles:    migrationFiles,
		AutoMigrate: runAutoMigrate,
		Seed:        runSeeders,
	})
}

func runAutoMigrate(_ context.Context, tx *gorm.DB) error {
	tx = tx.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci")

	modelsToMigrate := []any{
		// procyon:module-models
		&models.HelloMessage{},
	}
	if len(modelsToMigrate) == 0 {
		return nil
	}
	return tx.AutoMigrate(modelsToMigrate...)
}
