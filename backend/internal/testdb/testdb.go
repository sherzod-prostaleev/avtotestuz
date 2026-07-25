// Package testdb provides a migrated pool for integration tests.
//
// Each test *package* gets its own physical database, derived from the
// package's directory (internal/billing -> avtotest_test_internal_billing).
// That isolation is the whole point of this package: every New() truncates
// almost every table, so when packages shared one database a plain
// `go test ./...` had them wiping each other's fixtures mid-run. The damage
// surfaced as deadlocks and foreign-key violations inside unrelated tests —
// failures that look exactly like real concurrency bugs in the code under
// test and cost hours to chase. Correctness must not depend on remembering a
// command-line flag, so it no longer does.
//
// `make test` still passes -p 1, but only as a resource choice (one migration
// pass and one pool at a time); results are the same either way.
//
// Databases are created on demand and left behind between runs, since
// re-migrating from scratch on every run is the slow part. `make test-db-reset`
// drops them.
package testdb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db"
	"avtotest.uz/backend/internal/testenv"
)

// Postgres truncates identifiers at 63 bytes; anything longer would collide
// silently, so long names fall back to a hash suffix.
const maxIdentifierLen = 63

// Advisory-lock key guarding CREATE DATABASE. Concurrent creations from the
// same template can fail with "source database is being accessed by other
// users", so package setups queue here. Arbitrary constant, only has to be
// stable and unlikely to collide with application locks.
const createDatabaseLockKey int64 = 0x4176_546F_0001

var (
	setupOnce  sync.Once
	sharedPool *pgxpool.Pool
	setupErr   error
)

// baseURL is the DSN of the database that must already exist (created by
// `make up` / run.sh). It is used as the maintenance connection for CREATE
// DATABASE and as the template for per-package DSNs.
func baseURL() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable"
}

// databaseName appends the package suffix to the base database name, hashing
// instead of truncating when the result would exceed Postgres' identifier
// limit — a truncated name could be shared by two packages, quietly
// reintroducing exactly the cross-package interference this avoids.
func databaseName(base, suffix string) string {
	name := base + "_" + suffix
	if len(name) <= maxIdentifierLen {
		return name
	}
	sum := sha256.Sum256([]byte(suffix))
	short := hex.EncodeToString(sum[:])[:12]
	keep := maxIdentifierLen - len(short) - 1
	if keep > len(name) {
		keep = len(name)
	}
	return name[:keep] + "_" + short
}

// packageDSN returns this package's database name plus two DSNs for it.
//
// They differ by one query parameter and must not be interchanged:
// pool_max_conns is understood by pgxpool but golang-migrate forwards unknown
// parameters to the server as runtime settings, where it fails the connection
// outright with "unrecognized configuration parameter".
func packageDSN(base string) (name, migrateDSN, poolDSN string, err error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", "", "", fmt.Errorf("parse TEST_DATABASE_URL: %w", err)
	}
	baseName := strings.TrimPrefix(u.Path, "/")
	if baseName == "" {
		return "", "", "", errors.New("TEST_DATABASE_URL has no database name")
	}
	name = databaseName(baseName, testenv.PackageSlug())
	u.Path = "/" + name
	migrateDSN = u.String()

	// Cap the pool: with per-package databases nothing stops `go test ./...`
	// from starting every package at once, and the Postgres default of 100
	// connections is easy to exhaust with one default-sized pool per package.
	q := u.Query()
	if q.Get("pool_max_conns") == "" {
		q.Set("pool_max_conns", "6")
		u.RawQuery = q.Encode()
	}
	return name, migrateDSN, u.String(), nil
}

// ensureDatabase creates the per-package database if it does not exist yet,
// connecting to the base database to do so.
func ensureDatabase(ctx context.Context, base, name string) error {
	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		return fmt.Errorf("connect %s (run `make up` first): %w", base, err)
	}
	defer func() { _ = conn.Close(ctx) }()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, createDatabaseLockKey); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	// Released implicitly when the connection closes, but be explicit so a
	// slow migration in this package does not hold up the others.
	defer func() { _, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, createDatabaseLockKey) }()

	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		return fmt.Errorf("check database %s: %w", name, err)
	}
	if exists {
		return nil
	}

	// CREATE DATABASE takes no parameters, and the name is derived from our own
	// source tree rather than any input, so quoting it is enough.
	_, err = conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE %s`, pgx.Identifier{name}.Sanitize()))
	if err == nil {
		return nil
	}
	// 42P04 = duplicate_database: another package won the race between the
	// EXISTS check and here despite the lock (possible across separate
	// Postgres servers or if the lock was taken on a different database).
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P04" {
		return nil
	}
	return fmt.Errorf("create database %s: %w", name, err)
}

// New returns a pool to this package's migrated test database and truncates
// all tables.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	setupOnce.Do(func() { sharedPool, setupErr = setup() })
	if setupErr != nil {
		t.Fatalf("testdb setup: %v", setupErr)
	}
	Truncate(t, sharedPool)
	return sharedPool
}

func setup() (*pgxpool.Pool, error) {
	ctx := context.Background()
	base := baseURL()
	name, migrateDSN, poolDSN, err := packageDSN(base)
	if err != nil {
		return nil, err
	}
	if err := ensureDatabase(ctx, base, name); err != nil {
		return nil, err
	}
	if err := db.Migrate(migrateDSN); err != nil {
		return nil, fmt.Errorf("migrate %s: %w", name, err)
	}
	pool, err := db.NewPool(ctx, poolDSN)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", name, err)
	}
	return pool, nil
}

// Truncate wipes app tables. limit_config is intentionally kept (seeded config).
func Truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var err error
	for attempts := 0; attempts < 10; attempts++ {
		_, err = pool.Exec(context.Background(), `
			TRUNCATE TABLE
				notification, event, audit_log,
				streak, saved_question, category_mastery, question_memory,
				variant_progress, session_answer, exam_session,
				referral_attribution, referral, user_referral_code, promo_redemption, entitlement, payment,
				promo_code, tariff_translation, tariff,
				explanation_feedback, refresh_token, device, telegram_account, telegram_link_token,
				otp_challenge, profile,
				explanation_translation, explanation, question_sign,
				sign_translation, sign, sign_group_translation, sign_group,
				variant_question, variant, answer_translation, question_translation,
				answer, question, image, category_translation, category
			RESTART IDENTITY CASCADE`)
		if err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("truncate: %v", err)
}
