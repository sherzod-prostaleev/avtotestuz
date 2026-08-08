package db_test

import (
	"context"
	"encoding/json"
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
		"support_ticket", "support_conversation", "support_message", "alert_rule",
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

func TestAtomicAuditFallbackRollsBackMutationAndRedactsSecrets(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	var profileID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO profile (phone, name, password_hash)
		VALUES ('+998900009951', 'Before', 'must-not-enter-audit')
		RETURNING id`).Scan(&profileID); err != nil {
		t.Fatal(err)
	}

	var afterJSON []byte
	if err := pool.QueryRow(ctx, `
		SELECT after_json FROM admin_audit_log
		WHERE action = 'db.insert' AND entity_type = 'profile' AND entity_id = $1
		ORDER BY created_at DESC LIMIT 1`, profileID).Scan(&afterJSON); err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(afterJSON, &snapshot); err != nil {
		t.Fatal(err)
	}
	if _, exists := snapshot["password_hash"]; exists {
		t.Fatal("atomic audit snapshot must redact password_hash")
	}
	if _, err := pool.Exec(ctx, `
		CREATE FUNCTION test_reject_atomic_audit() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
		  IF NEW.action = 'db.update' THEN
		    RAISE EXCEPTION 'injected audit failure';
		  END IF;
		  RETURN NEW;
		END $$;
		CREATE TRIGGER test_reject_atomic_audit_trigger
		BEFORE INSERT ON admin_audit_log
		FOR EACH ROW EXECUTE FUNCTION test_reject_atomic_audit()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `
			DROP TRIGGER IF EXISTS test_reject_atomic_audit_trigger ON admin_audit_log;
			DROP FUNCTION IF EXISTS test_reject_atomic_audit()`)
	})

	if _, err := pool.Exec(ctx, `UPDATE profile SET name = 'After' WHERE id = $1`, profileID); err == nil {
		t.Fatal("mutation must fail when its same-transaction audit insert fails")
	}
	var name string
	if err := pool.QueryRow(ctx, `SELECT name FROM profile WHERE id = $1`, profileID).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Before" {
		t.Fatalf("profile mutation was not rolled back: name=%q", name)
	}
}

func TestAdminAuditAndContentRevisionAreAppendOnly(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	var auditID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO admin_audit_log (action, entity_type)
		VALUES ('test.append', 'test') RETURNING id`).Scan(&auditID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM admin_audit_log WHERE id = $1`, auditID); err == nil {
		t.Fatal("admin audit rows must be immutable")
	}

	var entityID, revisionID string
	if err := pool.QueryRow(ctx, `SELECT gen_random_uuid()`).Scan(&entityID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO content_revision (entity_type, entity_id, snapshot_json)
		VALUES ('test', $1, '{}') RETURNING id`, entityID).Scan(&revisionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE content_revision SET note = 'changed' WHERE id = $1`, revisionID); err == nil {
		t.Fatal("content revisions must be immutable")
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
		"tg_quiz_questions",                   // 0046
		"tg_quiz_seconds",                     // 0046
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
