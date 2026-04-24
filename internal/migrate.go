package internal

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/bartek5186/procyon/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:embed migrations/mysql/*.sql migrations/postgres/*.sql
var migrationFiles embed.FS

func MigrateRun(db *gorm.DB, cfg Config) error {
	if err := db.Transaction(func(tx *gorm.DB) error {
		if cfg.Database.DisableVersionedMigrations {
			if err := runAutoMigrate(tx); err != nil {
				return err
			}
		} else if err := runVersionedMigrations(tx, cfg); err != nil {
			return err
		}

		return seedHelloMessages(context.Background(), tx)
	}); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return nil
}

func runAutoMigrate(tx *gorm.DB) error {
	tx = tx.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci")

	return tx.AutoMigrate(
		&models.HelloMessage{},
	)
}

func runVersionedMigrations(tx *gorm.DB, cfg Config) error {
	driver := strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	if driver == "" {
		driver = "mysql"
	}
	if driver == "postgresql" {
		driver = "postgres"
	}
	if driver != "mysql" && driver != "postgres" {
		return fmt.Errorf("unsupported migration driver %q", driver)
	}

	table := strings.TrimSpace(cfg.Database.MigrationsTable)
	if table == "" {
		table = "schema_migrations"
	}
	if !isSafeSQLIdentifier(table) {
		return fmt.Errorf("invalid migrations table name %q", table)
	}

	if err := ensureMigrationsTable(tx, driver, table); err != nil {
		return err
	}

	applied, err := appliedMigrations(tx, table)
	if err != nil {
		return err
	}

	dir := "migrations/" + driver
	entries, err := fs.ReadDir(migrationFiles, dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version := strings.TrimSuffix(entry.Name(), ".sql")
		if applied[version] {
			continue
		}

		raw, err := migrationFiles.ReadFile(dir + "/" + entry.Name())
		if err != nil {
			return err
		}
		if err := execSQLStatements(tx, string(raw)); err != nil {
			return fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
		if err := tx.Exec(fmt.Sprintf("INSERT INTO %s (version) VALUES (?)", table), version).Error; err != nil {
			return err
		}
	}

	return nil
}

func ensureMigrationsTable(tx *gorm.DB, driver, table string) error {
	var statement string
	switch driver {
	case "mysql":
		statement = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			version VARCHAR(255) NOT NULL PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`, table)
	case "postgres":
		statement = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			version TEXT NOT NULL PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`, table)
	default:
		return fmt.Errorf("unsupported migration driver %q", driver)
	}

	return tx.Exec(statement).Error
}

func appliedMigrations(tx *gorm.DB, table string) (map[string]bool, error) {
	type row struct {
		Version string
	}

	var rows []row
	if err := tx.Raw(fmt.Sprintf("SELECT version FROM %s", table)).Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[string]bool, len(rows))
	for _, row := range rows {
		out[row.Version] = true
	}
	return out, nil
}

func execSQLStatements(tx *gorm.DB, raw string) error {
	lines := strings.Split(raw, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		cleaned = append(cleaned, line)
	}

	for _, stmt := range strings.Split(strings.Join(cleaned, "\n"), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if err := tx.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
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
