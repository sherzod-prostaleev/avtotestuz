package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TestMigrationPath0059To0062PreservesCoreData applies 0059→0060→0061→0062
// against an isolated DB that already holds core financial/support rows at v58.
// No production database is touched.
func TestMigrationPath0059To0062PreservesCoreData(t *testing.T) {
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
	drillName := fmt.Sprintf("%s_bpath_%s", baseName, strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
	if len(drillName) > 63 {
		drillName = "avtotest_bpath_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
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
	m, err := newMigrator(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Migrate(58); err != nil {
		t.Fatalf("migrate to 58: %v", err)
	}

	pool, err := NewPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var (
		profileID  uuid.UUID
		tariffID   uuid.UUID
		paymentID  uuid.UUID
		entitlementID uuid.UUID
		ticketID   uuid.UUID
		ledgerID   uuid.UUID
		paymeID    = "payme-mig-path-1"
		clickID    uuid.UUID
	)
	if err := pool.QueryRow(ctx, `
		INSERT INTO profile (phone, name, password_hash)
		VALUES ('+998901995901', 'Mig Path', 'hash')
		RETURNING id`).Scan(&profileID); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	var tariffDays int
	var tariffPrice int64
	if err := pool.QueryRow(ctx, `
		SELECT id, days, price_uzs FROM tariff ORDER BY sort_order LIMIT 1`).
		Scan(&tariffID, &tariffDays, &tariffPrice); err != nil {
		t.Fatalf("seed tariff lookup: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO payment (
		  profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		  tariff_days_snapshot, tariff_price_uzs_snapshot
		)
		VALUES ($1, $2, $3, 'payme', 'paid', 'mig-path-pay-1', $4, $5)
		RETURNING id`, profileID, tariffID, tariffPrice, tariffDays, tariffPrice).Scan(&paymentID); err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO payme_transaction (payme_id, payment_id, amount_tiyin, state, create_time)
		VALUES ($1, $2, 9900000, 2, $3)`, paymeID, paymentID, time.Now().UnixMilli()); err != nil {
		t.Fatalf("seed payme: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO click_transaction (click_trans_id, payment_id, amount_uzs, state)
		VALUES ('click-mig-1', $1, 99000, 1)
		RETURNING id`, paymentID).Scan(&clickID); err != nil {
		// payment already linked to payme; click may allow both — if unique on payment active fails, use separate payment.
		var pay2 uuid.UUID
		if err2 := pool.QueryRow(ctx, `
			INSERT INTO payment (
			  profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
			  tariff_days_snapshot, tariff_price_uzs_snapshot
			)
			VALUES ($1, $2, 49000, 'click', 'paid', 'mig-path-pay-2', $3, 49000)
			RETURNING id`, profileID, tariffID, tariffDays).Scan(&pay2); err2 != nil {
			t.Fatalf("seed click payment: %v (first click err: %v)", err2, err)
		}
		if err := pool.QueryRow(ctx, `
			INSERT INTO click_transaction (click_trans_id, payment_id, amount_uzs, state)
			VALUES ('click-mig-1', $1, 49000, 1)
			RETURNING id`, pay2).Scan(&clickID); err != nil {
			t.Fatalf("seed click: %v", err)
		}
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO entitlement (profile_id, source, starts_at, ends_at, payment_id)
		VALUES ($1, 'purchase', now() - interval '1 day', now() + interval '30 days', $2)
		RETURNING id`, profileID, paymentID).Scan(&entitlementID); err != nil {
		t.Fatalf("seed entitlement: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO referral_ledger (profile_id, entry_type, amount_uzs, payment_id)
		VALUES ($1, 'commission', 5000, $2)
		RETURNING id`, profileID, paymentID).Scan(&ledgerID); err != nil {
		t.Fatalf("seed referral_ledger: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO support_ticket (profile_id, subject, body, status)
		VALUES ($1, 'help', 'stuck', 'open')
		RETURNING id`, profileID).Scan(&ticketID); err != nil {
		t.Fatalf("seed support_ticket: %v", err)
	}

	assertVersion := func(want uint) {
		t.Helper()
		var version int
		var dirty bool
		if err := pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
			t.Fatal(err)
		}
		if uint(version) != want || dirty {
			t.Fatalf("schema version=%d dirty=%v, want %d/false", version, dirty, want)
		}
	}
	assertCoreIntact := func(label string) {
		t.Helper()
		var phone, payStatus, entSource, ticketStatus, ledgerType string
		var mustChange bool
		var paymeState, clickState int
		if err := pool.QueryRow(ctx, `SELECT phone, must_change_password FROM profile WHERE id=$1`, profileID).
			Scan(&phone, &mustChange); err != nil {
			// must_change_password only exists from 0060; tolerate pre-60 by probing.
			if err := pool.QueryRow(ctx, `SELECT phone FROM profile WHERE id=$1`, profileID).Scan(&phone); err != nil {
				t.Fatalf("%s profile: %v", label, err)
			}
		} else if mustChange {
			t.Fatalf("%s: existing profile must_change_password unexpectedly true", label)
		}
		if phone != "+998901995901" {
			t.Fatalf("%s phone=%q", label, phone)
		}
		if err := pool.QueryRow(ctx, `SELECT status FROM payment WHERE id=$1`, paymentID).Scan(&payStatus); err != nil {
			t.Fatalf("%s payment: %v", label, err)
		}
		if payStatus != "paid" {
			t.Fatalf("%s payment status=%s", label, payStatus)
		}
		if err := pool.QueryRow(ctx, `SELECT state FROM payme_transaction WHERE payme_id=$1`, paymeID).Scan(&paymeState); err != nil {
			t.Fatalf("%s payme: %v", label, err)
		}
		if paymeState != 2 {
			t.Fatalf("%s payme state=%d", label, paymeState)
		}
		if err := pool.QueryRow(ctx, `SELECT state FROM click_transaction WHERE id=$1`, clickID).Scan(&clickState); err != nil {
			t.Fatalf("%s click: %v", label, err)
		}
		if clickState != 1 {
			t.Fatalf("%s click state=%d", label, clickState)
		}
		if err := pool.QueryRow(ctx, `SELECT source FROM entitlement WHERE id=$1`, entitlementID).Scan(&entSource); err != nil {
			t.Fatalf("%s entitlement: %v", label, err)
		}
		if entSource != "purchase" {
			t.Fatalf("%s entitlement source=%s", label, entSource)
		}
		if err := pool.QueryRow(ctx, `SELECT entry_type FROM referral_ledger WHERE id=$1`, ledgerID).Scan(&ledgerType); err != nil {
			t.Fatalf("%s referral_ledger: %v", label, err)
		}
		if ledgerType != "commission" {
			t.Fatalf("%s ledger type=%s", label, ledgerType)
		}
		if err := pool.QueryRow(ctx, `SELECT status FROM support_ticket WHERE id=$1`, ticketID).Scan(&ticketStatus); err != nil {
			t.Fatalf("%s support_ticket: %v", label, err)
		}
		if ticketStatus != "open" {
			t.Fatalf("%s ticket status=%s", label, ticketStatus)
		}
	}

	assertCoreIntact("v58")

	for _, step := range []uint{59, 60, 61, 62} {
		if err := m.Migrate(step); err != nil {
			t.Fatalf("migrate to %d: %v", step, err)
		}
		assertVersion(step)
		assertCoreIntact(fmt.Sprintf("v%d", step))
	}

	// 0061 support chat tables exist; seed a conversation and ensure 0062 does not break it.
	var convID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO support_conversation (profile_id, status)
		VALUES ($1, 'open') RETURNING id`, profileID).Scan(&convID); err != nil {
		// Already at 62; conversation insert after loop — tables from 61.
		t.Fatalf("seed support_conversation after path: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO support_message (conversation_id, sender_kind, sender_profile_id, body)
		VALUES ($1, 'user', $2, 'hello after migrate')`, convID, profileID); err != nil {
		t.Fatalf("seed support_message: %v", err)
	}
	var msgCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM support_message WHERE conversation_id=$1`, convID).Scan(&msgCount); err != nil || msgCount != 1 {
		t.Fatalf("supportchat invariant: count=%d err=%v", msgCount, err)
	}

	for _, tbl := range []string{"broadcast_campaign", "broadcast_recipient"} {
		var reg *string
		if err := pool.QueryRow(ctx, `SELECT to_regclass($1)::text`, "public."+tbl).Scan(&reg); err != nil || reg == nil {
			t.Fatalf("0062 table %s missing", tbl)
		}
	}
	var hasCampaignCol bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM information_schema.columns
		  WHERE table_name='notification' AND column_name='campaign_id'
		)`).Scan(&hasCampaignCol); err != nil || !hasCampaignCol {
		t.Fatalf("notification.campaign_id missing: %v", err)
	}
}
