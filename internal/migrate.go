package internal

import (
	"embed"
	"fmt"
	"strings"

	coreconfig "github.com/bartek5186/procyon-core/config"
	"github.com/bartek5186/procyon/models"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

//go:embed migrations/mysql/* migrations/postgres/*
var migrationFiles embed.FS

func MigrateRun(db *gorm.DB, cfg coreconfig.Config) error {
	if cfg.AutoMigrateEnabled() {
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := runAutoMigrate(tx); err != nil {
				return err
			}
			return runSeeders(tx)
		}); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		return nil
	}

	if err := runGooseMigrations(db, cfg); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if err := runSeeders(db); err != nil {
		return fmt.Errorf("run seeders: %w", err)
	}

	return nil
}

func runGooseMigrations(db *gorm.DB, cfg coreconfig.Config) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	driver := migrationDriver(cfg)
	if err := goose.SetDialect(driver); err != nil {
		return err
	}

	table := strings.TrimSpace(cfg.Database.MigrationsTable)
	if table == "" {
		table = "schema_migrations"
	}
	if !isSafeSQLIdentifier(table) {
		return fmt.Errorf("invalid migrations table name %q", table)
	}
	goose.SetTableName(table)

	dir := strings.TrimSpace(cfg.Database.MigrationsDir)
	if dir == "" {
		dir = "migrations/" + driver
	}

	goose.SetBaseFS(migrationFiles)
	defer goose.SetBaseFS(nil)

	return goose.Up(sqlDB, dir)
}

func migrationDriver(cfg coreconfig.Config) string {
	driver := strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	switch driver {
	case "", "mysql":
		return "mysql"
	case "postgresql", "postgres":
		return "postgres"
	default:
		return driver
	}
}

func isSafeSQLIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_' {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func runAutoMigrate(tx *gorm.DB) error {
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
