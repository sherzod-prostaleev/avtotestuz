package db_test

import (
	"context"
	"testing"

	"avtotest.uz/backend/internal/testdb"
)

func TestMigrateCreatesAllTables(t *testing.T) {
	pool := testdb.New(t)
	for _, tbl := range []string{
		// content
		"category", "category_translation", "image", "question", "answer",
		"question_translation", "answer_translation", "variant", "variant_question",
		"sign_group", "sign", "sign_translation", "question_sign",
		"explanation", "explanation_translation",
		// identity
		"profile", "otp_challenge", "telegram_account", "device", "refresh_token",
		"explanation_feedback",
		// billing
		"tariff", "tariff_translation", "promo_code", "payment", "entitlement",
		"promo_redemption", "referral_attribution", "limit_config",
		// learning
		"exam_session", "session_answer", "variant_progress", "question_memory",
		"category_mastery", "saved_question", "streak",
		// system
		"audit_log", "event", "notification",
	} {
		var reg *string
		err := pool.QueryRow(context.Background(),
			"SELECT to_regclass($1)::text", "public."+tbl).Scan(&reg)
		if err != nil || reg == nil {
			t.Errorf("table %s missing (err=%v)", tbl, err)
		}
	}
}

func TestConstraintsAndSeeds(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM limit_config").Scan(&n); err != nil || n != 4 {
		t.Fatalf("limit_config seed count=%d err=%v, want 4", n, err)
	}
	// invalid locale must be rejected by domain
	if _, err := pool.Exec(ctx, "INSERT INTO category (code) VALUES ('x')"); err != nil {
		t.Fatalf("insert category: %v", err)
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO category_translation (category_id, locale, name)
		SELECT id, 'en', 'X' FROM category WHERE code='x'`)
	if err == nil {
		t.Fatal("want locale domain violation for 'en'")
	}
	// duplicate answer position must be rejected
	var qid string
	if err := pool.QueryRow(ctx, `
		INSERT INTO question (source_ext_id, category_id, content_hash)
		SELECT 'q-c', id, 'h' FROM category WHERE code='x' RETURNING id`).Scan(&qid); err != nil {
		t.Fatalf("insert question: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO answer (question_id, position) VALUES ($1, 1)`, qid); err != nil {
		t.Fatalf("insert answer: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO answer (question_id, position) VALUES ($1, 1)`, qid); err == nil {
		t.Fatal("want unique violation for duplicate answer position")
	}
}
