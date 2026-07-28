package db_test

import (
	"context"
	"slices"
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
		"profile", "otp_challenge", "telegram_account", "telegram_link_token",
		"telegram_chat", "telegram_quiz_session",
		"device", "refresh_token",
		"explanation_feedback",
		// billing
		"tariff", "tariff_translation", "promo_code", "payment", "entitlement",
		"promo_redemption", "referral", "user_referral_code", "limit_config", "payment_provider_status",
		// learning
		"exam_session", "session_question", "session_answer", "variant_progress", "question_memory",
		"category_mastery", "saved_question", "streak",
		// system
		"audit_log", "event", "notification", "push_subscription", "grand_mock_certificate",
		"support_ticket", "alert_rule",
	} {
		var reg *string
		err := pool.QueryRow(context.Background(),
			"SELECT to_regclass($1)::text", "public."+tbl).Scan(&reg)
		if err != nil || reg == nil {
			t.Errorf("table %s missing (err=%v)", tbl, err)
		}
	}
	// U-23: dead parallel table from 0003 must be gone; live path is referral (0015).
	var dead *string
	if err := pool.QueryRow(context.Background(),
		"SELECT to_regclass($1)::text", "public.referral_attribution").Scan(&dead); err != nil {
		t.Fatalf("referral_attribution probe: %v", err)
	}
	if dead != nil {
		t.Errorf("referral_attribution still present (%s); expected dropped by 0028", *dead)
	}
}

func TestConstraintsAndSeeds(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	// Assert the exact seeded key set rather than a bare count: a count told
	// us only "the number moved" and had to be hand-bumped three times, each
	// time leaving CI red until someone guessed which migration did it. The
	// set makes the failure name the offending key, so adding a seed is a
	// one-line, obvious update here.
	wantLimitKeys := []string{
		"daily_goal_default",                  // 0003
		"daily_practice_questions",            // 0003
		"grand_mock_min_studied_pct",          // 0018
		"grand_mock_threshold_pct",            // 0003
		"leaderboard_daily_points",            // 0017
		"referral_attach_window_days",         // 0024
		"referral_commission_percent_default", // 0041
		"unlock_threshold_correct",            // 0003
	}
	rows, err := pool.Query(ctx, "SELECT key FROM limit_config ORDER BY key")
	if err != nil {
		t.Fatalf("select limit_config: %v", err)
	}
	var gotLimitKeys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatalf("scan limit_config key: %v", err)
		}
		gotLimitKeys = append(gotLimitKeys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate limit_config: %v", err)
	}
	if !slices.Equal(gotLimitKeys, wantLimitKeys) {
		t.Fatalf("limit_config keys = %v, want %v", gotLimitKeys, wantLimitKeys)
	}
	// invalid locale must be rejected by domain
	if _, err := pool.Exec(ctx, "INSERT INTO category (code) VALUES ('x')"); err != nil {
		t.Fatalf("insert category: %v", err)
	}
	_, err = pool.Exec(ctx, `
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
