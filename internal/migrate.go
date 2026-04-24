package internal

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"github.com/bartek5186/procyon/models"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:embed migrations/mysql/*.sql migrations/postgres/*.sql
var migrationFiles embed.FS

func MigrateRun(db *gorm.DB, cfg Config) error {
	if cfg.Database.DisableVersionedMigrations {
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := runAutoMigrate(tx); err != nil {
				return err
			}
			return seedHelloMessages(context.Background(), tx)
		}); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		return nil
	}

	if err := runGooseMigrations(db, cfg); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	if err := seedHelloMessages(context.Background(), db); err != nil {
		return fmt.Errorf("seed hello messages: %w", err)
	}

	return nil
}

func runGooseMigrations(db *gorm.DB, cfg Config) error {
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

func migrationDriver(cfg Config) string {
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

	return tx.AutoMigrate(
		&models.HelloMessage{},
	)
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
