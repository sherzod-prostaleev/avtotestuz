package billing

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

func setupReferrerPair(t *testing.T, pool *pgxpool.Pool, referrerPhone, refereePhone string, percent int32) (referrer, referee uuid.UUID, svc Service) {
	t.Helper()
	ctx := context.Background()
	referrer = uuid.New()
	referee = uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone, referral_commission_percent) VALUES ($1, $2, $3)`, referrer, referrerPhone, percent); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, $2)`, referee, refereePhone); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tariff (code, days, price_uzs, sort_order, active)
		VALUES ('nexia', 7, 24900, 0, true)
		ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 24900, days = 7
	`); err != nil {
		t.Fatal(err)
	}
	svc = Service{Q: sqlc.New(pool), Pool: pool, Secret: []byte("test-wallet-audit-pan-secret-32-bytes")}
	code, err := svc.GetOrCreateReferralCode(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyReferralCode(ctx, referee, code); err != nil {
		t.Fatal(err)
	}
	return referrer, referee, svc
}

func payReferee(t *testing.T, pool *pgxpool.Pool, svc Service, referee, paymentID uuid.UUID, amount int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, paid_at, idempotency_key,
		                     tariff_days_snapshot, tariff_price_uzs_snapshot)
		SELECT $1, $2, t.id, $3, 'payme', 'paid', NOW(), $4, t.days, t.price_uzs
		FROM tariff t WHERE t.code = 'nexia'
	`, paymentID, referee, amount, uuid.New().String()); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProcessPaymentGrant(ctx, paymentID); err != nil {
		t.Fatal(err)
	}
}

func ledgerSum(t *testing.T, pool *pgxpool.Pool, profileID uuid.UUID) int64 {
	t.Helper()
	var sum int64
	if err := pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(amount_uzs), 0)::bigint FROM referral_ledger WHERE profile_id = $1`,
		profileID,
	).Scan(&sum); err != nil {
		t.Fatal(err)
	}
	return sum
}

func insertAdmin(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO admin_user (id, email, display_name, password_hash, status)
		 VALUES ($1, $2, 'Audit Admin', 'x', 'active')`,
		id, "audit-"+id.String()+"@test.local",
	); err != nil {
		t.Fatal(err)
	}
	return id
}

func markPayoutPaid(t *testing.T, pool *pgxpool.Pool, payoutID, adminID uuid.UUID, note string) error {
	t.Helper()
	ctx := context.Background()
	q := sqlc.New(pool)
	payout, err := q.GetReferralPayoutForUpdate(ctx, payoutID)
	if err != nil {
		return err
	}
	if payout.Status != "pending" {
		return ErrPayoutNotPending
	}
	_, err = q.MarkReferralPayoutPaid(ctx, sqlc.MarkReferralPayoutPaidParams{
		ID:          payoutID,
		ProcessedBy: uuid.NullUUID{UUID: adminID, Valid: true},
		AdminNote:   note,
	})
	return err
}

func rejectPayout(t *testing.T, pool *pgxpool.Pool, payoutID, adminID uuid.UUID, note string) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)
	payout, err := q.GetReferralPayoutForUpdate(ctx, payoutID)
	if err != nil {
		return err
	}
	if payout.Status != "pending" {
		return ErrPayoutNotPending
	}
	if _, err := q.MarkReferralPayoutRejected(ctx, sqlc.MarkReferralPayoutRejectedParams{
		ID:          payoutID,
		ProcessedBy: uuid.NullUUID{UUID: adminID, Valid: true},
		AdminNote:   note,
	}); err != nil {
		return err
	}
	meta, _ := json.Marshal(map[string]any{"reason": "rejected"})
	if _, err := q.InsertReferralLedger(ctx, sqlc.InsertReferralLedgerParams{
		ProfileID: payout.ProfileID,
		EntryType: "payout_reject_release",
		AmountUzs: payout.AmountUzs,
		PaymentID: uuid.NullUUID{},
		PayoutID:  uuid.NullUUID{UUID: payout.ID, Valid: true},
		Meta:      meta,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func TestAudit_CommissionPercentOverride(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer, referee, svc := setupReferrerPair(t, pool, "+998901300001", "+998901300002", 50)
	paymentID := uuid.New()
	payReferee(t, pool, svc, referee, paymentID, 24900)

	want := int64(24900 * 50 / 100)
	bal, err := svc.ReferralBalance(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if bal != want {
		t.Fatalf("balance=%d, want override commission %d", bal, want)
	}
	act, err := svc.GetReferralActivity(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if len(act.Earnings) != 1 {
		t.Fatalf("earnings=%d, want 1", len(act.Earnings))
	}
	if act.Earnings[0].PercentSnapshot != 50 || act.Earnings[0].CommissionUzs != want {
		t.Errorf("earning percent=%d commission=%d, want 50 / %d",
			act.Earnings[0].PercentSnapshot, act.Earnings[0].CommissionUzs, want)
	}
	if act.Earnings[0].TariffCode != "nexia" || act.Earnings[0].PaymentAmountUzs != 24900 {
		t.Errorf("tariff/payment mismatch: %+v", act.Earnings[0])
	}
	if bal != ledgerSum(t, pool, referrer) {
		t.Errorf("balance != ledger sum")
	}
}

func TestAudit_NoDoubleCommissionSamePayment(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer, referee, svc := setupReferrerPair(t, pool, "+998901300003", "+998901300004", 20)
	paymentID := uuid.New()
	payReferee(t, pool, svc, referee, paymentID, 24900)

	if err := svc.creditReferralCommission(ctx, referrer, referee, paymentID, 24900, 20, 7); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM referral_ledger WHERE profile_id = $1 AND entry_type = 'commission' AND payment_id = $2`,
		referrer, paymentID,
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("commission rows for payment=%d, want 1", n)
	}
	want := int64(24900 * 20 / 100)
	bal, _ := svc.ReferralBalance(ctx, referrer)
	if bal != want {
		t.Fatalf("balance=%d, want %d", bal, want)
	}
}

func TestAudit_PayoutHoldRejectPaidInvariants(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer, referee, svc := setupReferrerPair(t, pool, "+998901300005", "+998901300006", 20)
	paymentID := uuid.New()
	payReferee(t, pool, svc, referee, paymentID, 24900)
	want := int64(24900 * 20 / 100)

	payout, err := svc.RequestReferralPayout(ctx, referrer, want, "8600123456789012", "uzcard")
	if err != nil {
		t.Fatal(err)
	}
	balHold, _ := svc.ReferralBalance(ctx, referrer)
	if balHold != 0 {
		t.Fatalf("after hold balance=%d, want 0", balHold)
	}
	act, err := svc.GetReferralActivity(ctx, referrer)
	if err != nil {
		t.Fatal(err)
	}
	if act.PayoutSummary.PendingCount != 1 || act.PayoutSummary.PendingUzs != want {
		t.Fatalf("pending summary=%+v, want count=1 uzs=%d", act.PayoutSummary, want)
	}
	if len(act.Payouts) != 1 || act.Payouts[0].Status != "pending" {
		t.Fatalf("payouts=%+v", act.Payouts)
	}

	adminID := insertAdmin(t, pool)
	if err := rejectPayout(t, pool, payout.ID, adminID, "test reject"); err != nil {
		t.Fatal(err)
	}
	balReject, _ := svc.ReferralBalance(ctx, referrer)
	if balReject != want {
		t.Fatalf("after reject balance=%d, want %d", balReject, want)
	}
	act2, _ := svc.GetReferralActivity(ctx, referrer)
	if act2.PayoutSummary.RejectedCount != 1 || act2.PayoutSummary.PendingCount != 0 {
		t.Fatalf("after reject summary=%+v", act2.PayoutSummary)
	}

	payout2, err := svc.RequestReferralPayout(ctx, referrer, want, "8600123456789012", "uzcard")
	if err != nil {
		t.Fatal(err)
	}
	if err := markPayoutPaid(t, pool, payout2.ID, adminID, "paid ok"); err != nil {
		t.Fatal(err)
	}
	balPaid, _ := svc.ReferralBalance(ctx, referrer)
	if balPaid != 0 {
		t.Fatalf("after paid balance=%d, want 0 (hold already deducted)", balPaid)
	}
	if err := markPayoutPaid(t, pool, payout2.ID, adminID, "again"); err == nil {
		t.Fatal("expected not-pending error on second paid")
	}
	var holdCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM referral_ledger WHERE payout_id = $1 AND entry_type = 'payout_hold'`,
		payout2.ID,
	).Scan(&holdCount); err != nil {
		t.Fatal(err)
	}
	if holdCount != 1 {
		t.Fatalf("payout_hold rows=%d, want 1 (no double debit on paid)", holdCount)
	}
	act3, _ := svc.GetReferralActivity(ctx, referrer)
	if act3.PayoutSummary.PaidCount != 1 || act3.PayoutSummary.PaidUzs != want {
		t.Fatalf("paid summary=%+v", act3.PayoutSummary)
	}
	if balPaid != ledgerSum(t, pool, referrer) {
		t.Errorf("balance != ledger sum after paid")
	}
}

func TestAudit_ClawbackOnRefund(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer, referee, svc := setupReferrerPair(t, pool, "+998901300007", "+998901300008", 20)
	paymentID := uuid.New()
	payReferee(t, pool, svc, referee, paymentID, 24900)

	if err := svc.ClawbackReferralCommissionOnRefund(ctx, paymentID); err != nil {
		t.Fatal(err)
	}
	bal, _ := svc.ReferralBalance(ctx, referrer)
	if bal != 0 {
		t.Fatalf("after clawback balance=%d, want 0", bal)
	}
	if err := svc.ClawbackReferralCommissionOnRefund(ctx, paymentID); err != nil {
		t.Fatal(err)
	}
	var clawN int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM referral_ledger WHERE payment_id = $1 AND entry_type = 'clawback'`,
		paymentID,
	).Scan(&clawN); err != nil {
		t.Fatal(err)
	}
	if clawN != 1 {
		t.Fatalf("clawback rows=%d, want 1", clawN)
	}
	if bal != ledgerSum(t, pool, referrer) {
		t.Errorf("balance != ledger sum after clawback")
	}
}

// TestAudit_ClawbackIsFullEvenWhenPayoutPending pins the fix for a cash
// extraction loop. The clawback used to be clamped to the current balance
// and skipped entirely at <= 0. A pending payout writes a negative
// payout_hold row, so simply *requesting* a withdrawal zeroed the balance
// and cancelled the clawback — "refer yourself, buy, request payout, refund
// the purchase" then kept both the refund and the commission, repeatably.
// The full commission must now come back, driving the balance negative,
// which correctly blocks further withdrawals until it is worked off.
func TestAudit_ClawbackIsFullEvenWhenPayoutPending(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer, referee, svc := setupReferrerPair(t, pool, "+998901300009", "+998901300010", 20)
	paymentID := uuid.New()
	payReferee(t, pool, svc, referee, paymentID, 24900)
	commission := int64(24900 * 20 / 100)
	holdAmt := commission / 2
	if _, err := svc.RequestReferralPayout(ctx, referrer, holdAmt, "8600312345678906", "uzcard"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClawbackReferralCommissionOnRefund(ctx, paymentID); err != nil {
		t.Fatal(err)
	}

	var clawAmt int64
	if err := pool.QueryRow(ctx,
		`SELECT amount_uzs FROM referral_ledger WHERE payment_id = $1 AND entry_type = 'clawback'`,
		paymentID,
	).Scan(&clawAmt); err != nil {
		t.Fatal(err)
	}
	if clawAmt != -commission {
		t.Fatalf("claw amount=%d, want the full -%d", clawAmt, commission)
	}

	bal, _ := svc.ReferralBalance(ctx, referrer)
	if want := -holdAmt; bal != want {
		t.Fatalf("balance after full clawback=%d, want %d", bal, want)
	}
	if bal != ledgerSum(t, pool, referrer) {
		t.Errorf("balance != ledger sum after clawback")
	}

	// The debt must actually stop further withdrawals.
	if _, err := svc.RequestReferralPayout(ctx, referrer, 1, "8600312345678906", "uzcard"); !errors.Is(err, ErrPayoutInsufficientBalance) {
		t.Fatalf("payout on a negative balance err=%v, want ErrPayoutInsufficientBalance", err)
	}
}

func TestAudit_GetReferralActivityEmpty(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	profileID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901300011')`, profileID); err != nil {
		t.Fatal(err)
	}
	svc := Service{Q: sqlc.New(pool), Pool: pool}
	act, err := svc.GetReferralActivity(ctx, profileID)
	if err != nil {
		t.Fatal(err)
	}
	if act.Payouts == nil || act.Earnings == nil {
		t.Fatal("expected non-nil empty slices for JSON arrays")
	}
	if len(act.Payouts) != 0 || len(act.Earnings) != 0 {
		t.Fatalf("want empty activity, got payouts=%d earnings=%d", len(act.Payouts), len(act.Earnings))
	}
	if act.PayoutSummary.PendingCount != 0 || act.PayoutSummary.PaidUzs != 0 {
		t.Fatalf("want zero summary, got %+v", act.PayoutSummary)
	}
}

func TestAudit_ConcurrentPayoutRequestsDoNotOverdraw(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	referrer, referee, svc := setupReferrerPair(t, pool, "+998901300012", "+998901300013", 20)
	paymentID := uuid.New()
	payReferee(t, pool, svc, referee, paymentID, 24900)
	want := int64(24900 * 20 / 100)

	const n = 6
	errs := make([]error, n)
	var wg sync.WaitGroup
	ready := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-ready
			_, errs[i] = svc.RequestReferralPayout(ctx, referrer, want, "8600123456789012", "uzcard")
		}(i)
	}
	close(ready)
	wg.Wait()

	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if err != ErrPayoutInsufficientBalance {
			t.Errorf("unexpected payout error: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("successful payouts=%d, want exactly 1", ok)
	}
	bal, _ := svc.ReferralBalance(ctx, referrer)
	if bal != 0 {
		t.Fatalf("balance after concurrent payouts=%d, want 0", bal)
	}
	var holds int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM referral_ledger WHERE profile_id = $1 AND entry_type = 'payout_hold'`,
		referrer,
	).Scan(&holds); err != nil {
		t.Fatal(err)
	}
	if holds != 1 {
		t.Fatalf("hold rows=%d, want 1", holds)
	}
	if bal != ledgerSum(t, pool, referrer) {
		t.Errorf("balance != ledger sum")
	}
}

func TestAudit_NoCommissionWithoutAttribution(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	buyer := uuid.New()
	paymentID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO profile (id, phone) VALUES ($1, '+998901300014')`, buyer); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tariff (code, days, price_uzs, sort_order, active)
		VALUES ('nexia', 7, 24900, 0, true)
		ON CONFLICT (code) DO UPDATE SET active = true, price_uzs = 24900, days = 7
	`); err != nil {
		t.Fatal(err)
	}
	svc := Service{Q: sqlc.New(pool), Pool: pool}
	payReferee(t, pool, svc, buyer, paymentID, 24900)
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM referral_ledger`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("ledger rows=%d without attribution, want 0", n)
	}
	if err := svc.ClawbackReferralCommissionOnRefund(ctx, paymentID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM referral_ledger`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("clawback without commission created rows=%d", n)
	}
}
