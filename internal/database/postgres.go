package database

import (
	"context"
	"fmt"
	"time"

	"remnafamily/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

var pool *pgxpool.Pool

// Connect establishes a connection to the PostgreSQL database
func Connect(ctx context.Context, yamlCfg *config.YAMLConfig) error {
	cfg := config.Cfg

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDB,
	)

	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return fmt.Errorf("parse connection string: %w", err)
	}

	// Configure pool settings from YAML
	poolConfig.MaxConns = int32(yamlCfg.Database.MaxConnections)
	poolConfig.MinConns = int32(yamlCfg.Database.MinConnections)
	poolConfig.MaxConnLifetime = time.Duration(yamlCfg.Database.MaxConnLifetimeMinutes) * time.Minute
	poolConfig.MaxConnIdleTime = time.Duration(yamlCfg.Database.MaxConnIdleMinutes) * time.Minute
	poolConfig.HealthCheckPeriod = time.Duration(yamlCfg.Database.HealthCheckPeriodSeconds) * time.Second

	pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	return nil
}

// Close closes the database connection pool
func Close() {
	if pool != nil {
		pool.Close()
	}
}

// GetPool returns the database connection pool
func GetPool() *pgxpool.Pool {
	return pool
}
