package internal

import (
	"context"

	"gorm.io/gorm"
)

type Seeder func(context.Context, *gorm.DB) error

var seeders []Seeder

func RegisterSeeder(seeder Seeder) {
	if seeder != nil {
		seeders = append(seeders, seeder)
	}
}

func runSeeders(ctx context.Context, db *gorm.DB) error {
	for _, seeder := range seeders {
		if err := seeder(ctx, db); err != nil {
			return err
		}
	}
	return nil
}
