package click

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

// profileOf looks up the profile_id owning paymentID, for entitlement
// assertions (billing.Service.Status needs a profile id, not a payment id).
// Mirrors payme/methods_test.go's profileOf.
func profileOf(t *testing.T, pool *pgxpool.Pool, paymentID uuid.UUID) uuid.UUID {
	t.Helper()
	var profileID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT profile_id FROM payment WHERE id = $1`, paymentID).Scan(&profileID); err != nil {
		t.Fatalf("lookup profile_id: %v", err)
	}
	return profileID
}

// seedClickPaymentWithTariffDays is seedClickPayment with a caller-chosen
// tariff.days, for TestComplete_GrantDaysFailureRollsBack: a huge days value
// overflows the int64-nanosecond time.Duration arithmetic in
// billing.Service.GrantDays (days * 24 * time.Hour wraps around to a
// negative duration), so the computed entitlement end lands before its
// start — deterministically tripping entitlement's `CHECK (ends_at >
// starts_at)` constraint on insert. Mirrors payme/methods_test.go's
// seedPaymentWithTariffDays.
func seedClickPaymentWithTariffDays(t *testing.T, pool *pgxpool.Pool, status string, days int32) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	profileID := uuid.New()
	phone := fmt.Sprintf("+9989%08d", int(profileID.ID())%100000000)
	if _, err := pool.Exec(ctx,
		`INSERT INTO profile (id, phone) VALUES ($1, $2)`, profileID, phone); err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	tariffID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, 'gentra-overflow', $2, 59900, 1, true)`,
		tariffID, days); err != nil {
		t.Fatalf("seed tariff: %v", err)
	}

	paymentID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                      tariff_days_snapshot, tariff_price_uzs_snapshot)
		 SELECT $1, $2, t.id, 59900, 'click', $4, $5, t.days, t.price_uzs
		 FROM tariff t WHERE t.id = $3`,
		paymentID, profileID, tariffID, status, uuid.New().String()); err != nil {
		t.Fatalf("seed payment: %v", err)
	}

	return paymentID
}

// seedClickTransaction inserts a click_transaction row directly (bypassing
// prepare), so Complete tests can seed a known state=0 transaction pointed
// at a given payment/amount. Mirrors payme/methods_test.go's
// seedPaymeTransaction. Returns the new transaction's id.
func seedClickTransaction(t *testing.T, pool *pgxpool.Pool, clickTransID string, paymentID uuid.UUID, amountUzs int64) uuid.UUID {
	t.Helper()
	txID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO click_transaction (id, click_trans_id, payment_id, amount_uzs, state)
		 VALUES ($1, $2, $3, $4, 0)`,
		txID, clickTransID, paymentID, amountUzs); err != nil {
		t.Fatalf("seed click_transaction: %v", err)
	}
	return txID
}

// completeRequest builds a base Complete (action=1) clickRequest for the
// given transaction/payment/amount, with error="0" (Click's own "no error")
// unless overridden by the caller.
func completeRequest(clickTransID string, transactionID, paymentID uuid.UUID, amount string) clickRequest {
	return clickRequest{
		ClickTransID:      clickTransID,
		ServiceID:         methodsTestServiceID,
		MerchantTransID:   paymentID.String(),
		Amount:            amount,
		Action:            "1",
		Error:             "0",
		SignTime:          "1700000000",
		MerchantPrepareID: transactionID.String(),
	}
}

const methodsTestServiceID = "S"
const methodsTestSecretKey = "click-test-secret"

// newClickHandler builds a Handler wired to pool, ready to drive through
// ServeHTTP (exercising the full dispatch, not the method funcs directly) —
// mirrors payme/methods_test.go's newMethodsHandler.
func newClickHandler(pool *pgxpool.Pool) *Handler {
	q := sqlc.New(pool)
	return &Handler{
		Q:         q,
		Svc:       billing.Service{Q: q},
		Pool:      pool,
		ServiceID: methodsTestServiceID,
		SecretKey: methodsTestSecretKey,
	}
}

// seedClickPayment inserts a profile + tariff (price 59900 so'm) + a payment
// row in the given status, and returns the payment id — mirrors
// payme/methods_test.go's seedPayment, but with provider='click'.
func seedClickPayment(t *testing.T, pool *pgxpool.Pool, status string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	profileID := uuid.New()
	phone := fmt.Sprintf("+9989%08d", int(profileID.ID())%100000000)
	if _, err := pool.Exec(ctx,
		`INSERT INTO profile (id, phone) VALUES ($1, $2)`, profileID, phone); err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	tariffID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, 'gentra', 30, 59900, 1, true)`,
		tariffID); err != nil {
		t.Fatalf("seed tariff: %v", err)
	}

	paymentID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key,
		                      tariff_days_snapshot, tariff_price_uzs_snapshot)
		 SELECT $1, $2, t.id, 59900, 'click', $4, $5, t.days, t.price_uzs
		 FROM tariff t WHERE t.id = $3`,
		paymentID, profileID, tariffID, status, uuid.New().String()); err != nil {
		t.Fatalf("seed payment: %v", err)
	}

	return paymentID
}

// clickCall builds a correctly-signed prepare (action=0) request via
// computeSign, drives it through h.ServeHTTP, and decodes the response body.
func clickCall(t *testing.T, h *Handler, req clickRequest) clickResponse {
	t.Helper()
	req.SignString = computeSign(req, h.ServiceID, h.SecretKey)

	vals := url.Values{
		"click_trans_id":      {req.ClickTransID},
		"service_id":          {req.ServiceID},
		"click_paydoc_id":     {req.ClickPaydocID},
		"merchant_trans_id":   {req.MerchantTransID},
		"amount":              {req.Amount},
		"action":              {req.Action},
		"error":               {req.Error},
		"error_note":          {req.ErrorNote},
		"sign_time":           {req.SignTime},
		"sign_string":         {req.SignString},
		"merchant_prepare_id": {req.MerchantPrepareID},
	}

	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/billing/click", strings.NewReader(vals.Encode()))
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, httpReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var resp clickResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
	}
	return resp
}

// prepareRequest builds a base prepare (action=0) clickRequest for the given
// paymentID/clickTransID/amount, with error="0" (Click's own "no error")
// unless overridden by the caller.
func prepareRequest(clickTransID, merchantTransID, amount string) clickRequest {
	return clickRequest{
		ClickTransID:    clickTransID,
		ServiceID:       methodsTestServiceID,
		MerchantTransID: merchantTransID,
		Amount:          amount,
		Action:          "0",
		Error:           "0",
		SignTime:        "1700000000",
	}
}

func TestPrepare_Success(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "created")

	resp := clickCall(t, h, prepareRequest("click-txn-1", paymentID.String(), "59900"))

	if resp.Error != errSuccess {
		t.Fatalf("error = %d, want %d (note=%q)", resp.Error, errSuccess, resp.ErrorNote)
	}
	if resp.ErrorNote != "Success" {
		t.Errorf("error_note = %q, want %q", resp.ErrorNote, "Success")
	}
	if resp.MerchantPrepareID == "" {
		t.Fatalf("merchant_prepare_id = %q, want non-empty uuid", resp.MerchantPrepareID)
	}
	prepareID, err := uuid.Parse(resp.MerchantPrepareID)
	if err != nil {
		t.Fatalf("merchant_prepare_id = %q, not a valid uuid: %v", resp.MerchantPrepareID, err)
	}

	var state int32
	var gotClickTransID string
	if err := pool.QueryRow(context.Background(),
		`SELECT state, click_trans_id FROM click_transaction WHERE id = $1`, prepareID).
		Scan(&state, &gotClickTransID); err != nil {
		t.Fatalf("query click_transaction: %v", err)
	}
	if state != 0 {
		t.Errorf("click_transaction.state = %d, want 0", state)
	}
	if gotClickTransID != "click-txn-1" {
		t.Errorf("click_transaction.click_trans_id = %q, want %q", gotClickTransID, "click-txn-1")
	}

	var status string
	var providerTxnID *string
	if err := pool.QueryRow(context.Background(),
		`SELECT status, provider_txn_id FROM payment WHERE id = $1`, paymentID).
		Scan(&status, &providerTxnID); err != nil {
		t.Fatalf("query payment: %v", err)
	}
	if status != "pending" {
		t.Errorf("payment.status = %q, want pending", status)
	}
	if providerTxnID == nil || *providerTxnID != "click-txn-1" {
		t.Errorf("payment.provider_txn_id = %v, want %q", providerTxnID, "click-txn-1")
	}
}

func TestPrepare_WrongAmount(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "created")

	resp := clickCall(t, h, prepareRequest("click-txn-2", paymentID.String(), "1"))

	if resp.Error != errAmount {
		t.Errorf("error = %d, want %d", resp.Error, errAmount)
	}
}

func TestPrepare_UnknownMerchantTransID(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	_ = seedClickPayment(t, pool, "created")

	resp := clickCall(t, h, prepareRequest("click-txn-3", uuid.New().String(), "59900"))

	if resp.Error != errAccountNotFound {
		t.Errorf("error = %d, want %d", resp.Error, errAccountNotFound)
	}
}

func TestPrepare_MalformedMerchantTransID(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)

	resp := clickCall(t, h, prepareRequest("click-txn-4", "not-a-uuid", "59900"))

	if resp.Error != errAccountNotFound {
		t.Errorf("error = %d, want %d", resp.Error, errAccountNotFound)
	}
}

func TestPrepare_AlreadyPaid(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "paid")

	resp := clickCall(t, h, prepareRequest("click-txn-5", paymentID.String(), "59900"))

	if resp.Error != errAlreadyPaid {
		t.Errorf("error = %d, want %d", resp.Error, errAlreadyPaid)
	}
}

func TestPrepare_ClickSentError(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "created")

	req := prepareRequest("click-txn-6", paymentID.String(), "59900")
	req.Error = "-1"
	resp := clickCall(t, h, req)

	if resp.Error != errCancelled {
		t.Errorf("error = %d, want %d", resp.Error, errCancelled)
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM click_transaction WHERE click_trans_id = $1`, "click-txn-6").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("click_transaction row count = %d, want 0 (cancelled prepare must not insert)", count)
	}
}

func TestPrepare_IdempotentReplay(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "created")

	first := clickCall(t, h, prepareRequest("click-txn-7", paymentID.String(), "59900"))
	if first.Error != errSuccess {
		t.Fatalf("first prepare error = %d, want %d", first.Error, errSuccess)
	}

	second := clickCall(t, h, prepareRequest("click-txn-7", paymentID.String(), "59900"))
	if second.Error != errSuccess {
		t.Fatalf("second prepare error = %d, want %d", second.Error, errSuccess)
	}

	if second.MerchantPrepareID != first.MerchantPrepareID {
		t.Errorf("replay merchant_prepare_id = %q, want same as first %q", second.MerchantPrepareID, first.MerchantPrepareID)
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM click_transaction WHERE click_trans_id = $1`, "click-txn-7").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("click_transaction row count = %d, want 1 (no duplicate insert on replay)", count)
	}
}

func TestPrepare_ConflictActiveTransaction(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "created")

	first := clickCall(t, h, prepareRequest("click-txn-8a", paymentID.String(), "59900"))
	if first.Error != errSuccess {
		t.Fatalf("first prepare error = %d, want %d", first.Error, errSuccess)
	}

	second := clickCall(t, h, prepareRequest("click-txn-8b", paymentID.String(), "59900"))
	if second.Error != errBadRequest {
		t.Errorf("error = %d, want %d", second.Error, errBadRequest)
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM click_transaction WHERE click_trans_id = $1`, "click-txn-8b").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("click_transaction row count for click-txn-8b = %d, want 0 (conflicting prepare must not insert)", count)
	}
}

// TestComplete_Success proves the whole confirm+grant sequence: state flips
// to 1, confirmed_at is set, payment is marked paid, and — critically —
// billing.Service.Status reports an active entitlement, proving GrantDays
// genuinely ran end-to-end rather than the HTTP call merely returning 200.
func TestComplete_Success(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "pending")
	txID := seedClickTransaction(t, pool, "click-complete-1", paymentID, 59900)

	resp := clickCall(t, h, completeRequest("click-complete-1", txID, paymentID, "59900"))

	if resp.Error != errSuccess {
		t.Fatalf("error = %d, want %d (note=%q)", resp.Error, errSuccess, resp.ErrorNote)
	}
	if resp.MerchantConfirmID != txID.String() {
		t.Errorf("merchant_confirm_id = %q, want %q", resp.MerchantConfirmID, txID.String())
	}

	ctx := context.Background()
	var state int32
	var confirmedAt *string
	if err := pool.QueryRow(ctx,
		`SELECT state, confirmed_at::text FROM click_transaction WHERE id = $1`, txID).
		Scan(&state, &confirmedAt); err != nil {
		t.Fatalf("query click_transaction: %v", err)
	}
	if state != 1 {
		t.Errorf("click_transaction.state = %d, want 1", state)
	}
	if confirmedAt == nil {
		t.Errorf("click_transaction.confirmed_at = nil, want set")
	}

	var status string
	var paidAt *string
	if err := pool.QueryRow(ctx,
		`SELECT status, paid_at::text FROM payment WHERE id = $1`, paymentID).
		Scan(&status, &paidAt); err != nil {
		t.Fatalf("query payment: %v", err)
	}
	if status != "paid" {
		t.Errorf("payment.status = %q, want paid", status)
	}
	if paidAt == nil {
		t.Errorf("payment.paid_at = nil, want set")
	}

	profileID := profileOf(t, pool, paymentID)
	active, until, err := h.Svc.Status(ctx, profileID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !active {
		t.Errorf("entitlement active = false, want true (GrantDays should have run)")
	}
	if until == nil {
		t.Errorf("entitlement until = nil, want set")
	}
}

// TestComplete_IdempotentReplay proves a retried Complete for an
// already-paid transaction returns -4 (Click's real "already paid"
// semantics for this case) without re-granting entitlement: the
// entitlement `until` timestamp from Status() must be unchanged between
// the two calls.
func TestComplete_IdempotentReplay(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "pending")
	txID := seedClickTransaction(t, pool, "click-complete-2", paymentID, 59900)

	first := clickCall(t, h, completeRequest("click-complete-2", txID, paymentID, "59900"))
	if first.Error != errSuccess {
		t.Fatalf("first complete error = %d, want %d (note=%q)", first.Error, errSuccess, first.ErrorNote)
	}

	ctx := context.Background()
	profileID := profileOf(t, pool, paymentID)
	_, until1, err := h.Svc.Status(ctx, profileID)
	if err != nil {
		t.Fatalf("Status (1st): %v", err)
	}

	second := clickCall(t, h, completeRequest("click-complete-2", txID, paymentID, "59900"))
	if second.Error != errAlreadyPaid {
		t.Errorf("second complete error = %d, want %d", second.Error, errAlreadyPaid)
	}

	_, until2, err := h.Svc.Status(ctx, profileID)
	if err != nil {
		t.Fatalf("Status (2nd): %v", err)
	}
	if until1 == nil || until2 == nil || !until1.Equal(*until2) {
		t.Errorf("entitlement until changed across replay: 1st=%v 2nd=%v (GrantDays must not re-run)", until1, until2)
	}
}

// TestComplete_UnknownMerchantPrepareID proves an unresolvable
// merchant_prepare_id (malformed or not found) is reported as -6.
func TestComplete_UnknownMerchantPrepareID(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "pending")

	resp := clickCall(t, h, completeRequest("click-complete-3", uuid.New(), paymentID, "59900"))

	if resp.Error != errTransactionNotFound {
		t.Errorf("error = %d, want %d", resp.Error, errTransactionNotFound)
	}
}

// TestComplete_MerchantTransIDMismatch proves a merchant_trans_id that
// doesn't match the transaction's own payment_id is reported as -5.
func TestComplete_MerchantTransIDMismatch(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "pending")
	txID := seedClickTransaction(t, pool, "click-complete-4", paymentID, 59900)

	req := completeRequest("click-complete-4", txID, paymentID, "59900")
	req.MerchantTransID = uuid.New().String()
	resp := clickCall(t, h, req)

	if resp.Error != errAccountNotFound {
		t.Errorf("error = %d, want %d", resp.Error, errAccountNotFound)
	}
}

// TestComplete_WrongAmount proves an amount mismatch against the
// transaction's own amount_uzs is reported as -2.
func TestComplete_WrongAmount(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "pending")
	txID := seedClickTransaction(t, pool, "click-complete-5", paymentID, 59900)

	resp := clickCall(t, h, completeRequest("click-complete-5", txID, paymentID, "1"))

	if resp.Error != errAmount {
		t.Errorf("error = %d, want %d", resp.Error, errAmount)
	}
}

// TestComplete_ClickSentError proves Click's own decline (error<0 on the
// Complete call) is reported as -9, and rejectTransaction runs atomically:
// click_transaction.state=-1, rejected_at set, reason=error_note, and
// payment.status='canceled'.
func TestComplete_ClickSentError(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "pending")
	txID := seedClickTransaction(t, pool, "click-complete-6", paymentID, 59900)

	req := completeRequest("click-complete-6", txID, paymentID, "59900")
	req.Error = "-1"
	req.ErrorNote = "user declined"
	resp := clickCall(t, h, req)

	if resp.Error != errCancelled {
		t.Errorf("error = %d, want %d", resp.Error, errCancelled)
	}

	ctx := context.Background()
	var state int32
	var rejectedAt *string
	var reason *string
	if err := pool.QueryRow(ctx,
		`SELECT state, rejected_at::text, reason FROM click_transaction WHERE id = $1`, txID).
		Scan(&state, &rejectedAt, &reason); err != nil {
		t.Fatalf("query click_transaction: %v", err)
	}
	if state != -1 {
		t.Errorf("click_transaction.state = %d, want -1", state)
	}
	if rejectedAt == nil {
		t.Errorf("click_transaction.rejected_at = nil, want set")
	}
	if reason == nil || *reason != "user declined" {
		t.Errorf("click_transaction.reason = %v, want %q", reason, "user declined")
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM payment WHERE id = $1`, paymentID).Scan(&status); err != nil {
		t.Fatalf("query payment: %v", err)
	}
	if status != "canceled" {
		t.Errorf("payment.status = %q, want canceled", status)
	}
}

// TestComplete_GrantDaysFailureRollsBack proves confirmAndGrant's atomicity:
// if GrantDays fails after the state flip would otherwise have happened,
// the whole DB transaction must roll back, so click_transaction.state stays
// 0 (not 1) and payment.status stays 'pending' (not 'paid') — leaving room
// for a genuine retry to actually retry. Mirrors payme's
// TestPerformTransaction_GrantDaysFailureRollsBack.
func TestComplete_GrantDaysFailureRollsBack(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPaymentWithTariffDays(t, pool, "pending", 2000000000)
	txID := seedClickTransaction(t, pool, "click-complete-7", paymentID, 59900)

	resp := clickCall(t, h, completeRequest("click-complete-7", txID, paymentID, "59900"))

	if resp.Error == errSuccess {
		t.Fatalf("expected error (GrantDays should have failed), got success")
	}

	ctx := context.Background()
	var state int32
	if err := pool.QueryRow(ctx,
		`SELECT state FROM click_transaction WHERE id = $1`, txID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != 0 {
		t.Errorf("click_transaction.state = %d, want 0 (rolled back, not stranded at 1)", state)
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM payment WHERE id = $1`, paymentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Errorf("payment.status = %q, want pending", status)
	}
}
