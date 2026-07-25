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
		"profile", "otp_challenge", "telegram_account", "telegram_link_token", "device", "refresh_token",
		"explanation_feedback",
		// billing
		"tariff", "tariff_translation", "promo_code", "payment", "entitlement",
		"promo_redemption", "referral_attribution", "limit_config",
		// learning
		"exam_session", "session_question", "session_answer", "variant_progress", "question_memory",
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
	// 4 rows from 0003 + leaderboard_daily_points (0017) +
	// grand_mock_min_studied_pct (0018). Bump this when a migration seeds another.
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM limit_config").Scan(&n); err != nil || n != 6 {
		t.Fatalf("limit_config seed count=%d err=%v, want 6", n, err)
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

func TestSessionQuestionOrderingAndMembershipConstraints(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	var categoryID, questionA, questionB, answerA, answerB, profileID, sessionID string
	if err := pool.QueryRow(ctx, `INSERT INTO category (code) VALUES ('session-constraints') RETURNING id`).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO question (source_ext_id, category_id, content_hash)
		VALUES ('session-q-a', $1, 'session-h-a') RETURNING id`, categoryID).Scan(&questionA); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO question (source_ext_id, category_id, content_hash)
		VALUES ('session-q-b', $1, 'session-h-b') RETURNING id`, categoryID).Scan(&questionB); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO answer (question_id, position) VALUES ($1, 1) RETURNING id`, questionA).Scan(&answerA); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO answer (question_id, position) VALUES ($1, 1) RETURNING id`, questionB).Scan(&answerB); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO profile (phone) VALUES ('+998900000099') RETURNING id`).Scan(&profileID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO exam_session (profile_id, mode, locale, total)
		VALUES ($1, 'practice', 'uz-Latn', 1) RETURNING id`, profileID).Scan(&sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO session_question (session_id, question_id, position)
		VALUES ($1, $2, 1)`, sessionID, questionA); err != nil {
		t.Fatalf("insert assigned question: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO session_question (session_id, question_id, position)
		VALUES ($1, $2, 1)`, sessionID, questionB); err == nil {
		t.Fatal("duplicate position in one session must be rejected")
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO session_answer (session_id, question_id, answer_id, is_correct, position)
		VALUES ($1, $2, $3, false, 2)`, sessionID, questionB, answerB); err == nil {
		t.Fatal("answer for an unassigned question must be rejected by the composite FK")
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO session_answer (session_id, question_id, answer_id, is_correct, position)
		VALUES ($1, $2, $3, false, 1)`, sessionID, questionA, answerA); err != nil {
		t.Fatalf("answer for assigned question: %v", err)
	}
}
