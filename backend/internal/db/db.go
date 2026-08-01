// Package db owns the connection pool and schema migrations.
package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return NewPoolConfigured(ctx, databaseURL, PoolConfig{})
}

type PoolConfig struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

func NewPoolConfigured(ctx context.Context, databaseURL string, limits PoolConfig) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgxpool config: %w", err)
	}
	if limits.MaxConns > 0 {
		cfg.MaxConns = limits.MaxConns
	}
	if limits.MinConns >= 0 && limits.MaxConns > 0 {
		cfg.MinConns = limits.MinConns
	}
	if limits.MaxConnLifetime > 0 {
		cfg.MaxConnLifetime = limits.MaxConnLifetime
	}
	if limits.MaxConnIdleTime > 0 {
		cfg.MaxConnIdleTime = limits.MaxConnIdleTime
	}
	if limits.HealthCheckPeriod > 0 {
		cfg.HealthCheckPeriod = limits.HealthCheckPeriod
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return pool, nil
}

// Migrate applies all embedded migrations. Safe to call on every startup.
func Migrate(databaseURL string) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func newMigrator(databaseURL string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migrations fs: %w", err)
	}
	url := strings.Replace(databaseURL, "postgres://", "pgx5://", 1)
	m, err := migrate.NewWithSourceInstance("iofs", src, url)
	if err != nil {
		return nil, fmt.Errorf("migrate init: %w", err)
	}
	return m, nil
}

// MaintainEventPartitions pre-creates future event partitions. It never drops
// a partition or removes an event; retention remains an explicit product/data
// governance decision.
func MaintainEventPartitions(ctx context.Context, pool *pgxpool.Pool, monthsAhead int) (int, error) {
	if monthsAhead <= 0 {
		monthsAhead = 18
	}
	var created int
	if err := pool.QueryRow(ctx, `SELECT ensure_event_partitions($1)`, monthsAhead).Scan(&created); err != nil {
		return 0, fmt.Errorf("maintain event partitions: %w", err)
	}
	return created, nil
}
