// Package testdb provides a migrated pool for integration tests.
//
// All DB test packages share one database, so `go test` MUST run with -p 1
// (see Makefile/CI) — parallel packages would truncate each other's data.
package testdb

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db"
)

var migrateOnce sync.Once

func url() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://avtotest:avtotest@localhost:5432/avtotest_test?sslmode=disable"
}

// New returns a pool to the migrated test database and truncates all tables.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	migrateOnce.Do(func() {
		if err := db.Migrate(url()); err != nil {
			t.Fatalf("migrate test db: %v", err)
		}
	})
	pool, err := db.NewPool(context.Background(), url())
	if err != nil {
		t.Fatalf("connect test db: %v (run `make up` first)", err)
	}
	t.Cleanup(pool.Close)
	Truncate(t, pool)
	return pool
}

// Truncate wipes app tables. limit_config is intentionally kept (seeded config).
func Truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		TRUNCATE TABLE
			notification, event, audit_log,
			streak, saved_question, category_mastery, question_memory,
			variant_progress, session_answer, exam_session,
			referral_attribution, promo_redemption, entitlement, payment,
			promo_code, tariff_translation, tariff,
			explanation_feedback, refresh_token, device, telegram_account,
			otp_challenge, profile,
			explanation_translation, explanation, question_sign,
			sign_translation, sign, sign_group_translation, sign_group,
			variant_question, variant, answer_translation, question_translation,
			answer, question, image, category_translation, category
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}
