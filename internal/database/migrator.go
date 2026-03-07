package database

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"remnafamily/internal/config"
	"remnafamily/internal/utils"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// MigrateUp runs all pending database migrations
func MigrateUp(ctx context.Context) error {
	m, err := newMigrator()
	if err != nil {
		return err
	}
	defer m.Close()

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		utils.LogSystem("Database schema is up to date")
	} else {
		version, dirty, _ := m.Version()
		utils.LogSystem("Migrations applied successfully (version: %d, dirty: %v)", version, dirty)
	}

	return nil
}

// MigrateDown rolls back the last migration
func MigrateDown(ctx context.Context) error {
	m, err := newMigrator()
	if err != nil {
		return err
	}
	defer m.Close()

	err = m.Steps(-1)
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("rollback migration: %w", err)
	}

	version, dirty, _ := m.Version()
	utils.LogSystem("Migration rolled back (version: %d, dirty: %v)", version, dirty)

	return nil
}

// MigrateVersion returns the current migration version
func MigrateVersion(ctx context.Context) (uint, bool, error) {
	m, err := newMigrator()
	if err != nil {
		return 0, false, err
	}
	defer m.Close()

	return m.Version()
}

// MigrateForce forces a specific version (use with caution!)
func MigrateForce(ctx context.Context, version int) error {
	m, err := newMigrator()
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Force(version); err != nil {
		return fmt.Errorf("force migration version: %w", err)
	}

	utils.LogSystem("Forced migration version to %d", version)
	return nil
}

// newMigrator creates a new migrate instance
func newMigrator() (*migrate.Migrate, error) {
	cfg := config.Cfg

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDB,
	)

	// Create source driver from embedded filesystem
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("create migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, connStr)
	if err != nil {
		return nil, fmt.Errorf("create migrator: %w", err)
	}

	return m, nil
}
