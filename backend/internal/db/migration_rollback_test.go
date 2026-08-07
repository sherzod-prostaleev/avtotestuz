package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TestEveryMigrationDownAndUp uses an isolated disposable database. It never
// points a down migration at the shared/local application database.
func TestEveryMigrationDownAndUp(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		base = "postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable"
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	baseName := strings.TrimPrefix(u.Path, "/")
	if baseName == "" {
		t.Fatal("TEST_DATABASE_URL has no database name")
	}
	drillName := fmt.Sprintf("%s_rollback_%s", baseName, strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
	if len(drillName) > 63 {
		drillName = "avtotest_rollback_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	}

	ctx := context.Background()
	maintenance, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = maintenance.Close(ctx) }()
	quoted := pgx.Identifier{drillName}.Sanitize()
	if _, err := maintenance.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = maintenance.Exec(context.Background(), `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1`, drillName)
		_, _ = maintenance.Exec(context.Background(), "DROP DATABASE IF EXISTS "+quoted)
	})

	u.Path = "/" + drillName
	dsn := u.String()
	if err := Migrate(dsn); err != nil {
		t.Fatalf("initial up: %v", err)
	}
	m, err := newMigrator(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Down(); err != nil {
		_, _ = m.Close()
		t.Fatalf("all down migrations: %v", err)
	}
	_, _ = m.Close()

	if err := Migrate(dsn); err != nil {
		t.Fatalf("re-apply all up migrations: %v", err)
	}
	pool, err := NewPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var version int
	var dirty bool
	if err := pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatal(err)
	}
	if version != 58 || dirty {
		t.Fatalf("schema version=%d dirty=%v, want 58/false", version, dirty)
	}
}
