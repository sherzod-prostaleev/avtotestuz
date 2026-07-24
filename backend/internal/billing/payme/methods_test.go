package payme

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/testdb"
)

const methodsTestKey = "test-cashbox-key"

// newMethodsHandler builds a Handler wired to pool, ready to drive through
// ServeHTTP (exercising the full dispatch, not the method funcs directly).
func newMethodsHandler(pool *pgxpool.Pool) *Handler {
	q := sqlc.New(pool)
	return &Handler{Q: q, Svc: billing.Service{Q: q}, Pool: pool, Key: methodsTestKey}
}

// seedPaymeTransaction inserts a payme_transaction row directly (bypassing
// CreateTransaction), so PerformTransaction/CancelTransaction tests can
// control create_time precisely — e.g. to simulate a 12h-expired pending
// transaction, which CreateTransaction's own "now" clock can't produce.
func seedPaymeTransaction(t *testing.T, pool *pgxpool.Pool, paymeID string, paymentID uuid.UUID, state int32, createTime int64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO payme_transaction (payme_id, payment_id, amount_tiyin, state, create_time)
		 VALUES ($1, $2, 5990000, $3, $4)`,
		paymeID, paymentID, state, createTime); err != nil {
		t.Fatalf("seed payme_transaction: %v", err)
	}
}

// profileOf looks up the profile_id owning paymentID, for entitlement
// assertions (billing.Service.Status needs a profile id, not a payment id).
func profileOf(t *testing.T, pool *pgxpool.Pool, paymentID uuid.UUID) uuid.UUID {
	t.Helper()
	var profileID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT profile_id FROM payment WHERE id = $1`, paymentID).Scan(&profileID); err != nil {
		t.Fatalf("lookup profile_id: %v", err)
	}
	return profileID
}

// seedPayment inserts a profile + tariff (price 59900 so'm) + a payment row
// in the given status, and returns the payment id.
func seedPayment(t *testing.T, pool *pgxpool.Pool, status string) uuid.UUID {
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
		`INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key)
		 VALUES ($1, $2, $3, 59900, 'payme', $4, $5)`,
		paymentID, profileID, tariffID, status, uuid.New().String()); err != nil {
		t.Fatalf("seed payment: %v", err)
	}

	return paymentID
}

// seedPaymentTariffCode is seedPayment with a caller-chosen tariff code, for
// tests (like GetStatement's) that seed more than one payment in a single
// database: seedPayment's hardcoded 'gentra' code would collide with itself
// on a second call within the same test.
func seedPaymentTariffCode(t *testing.T, pool *pgxpool.Pool, status, tariffCode string) uuid.UUID {
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
		`INSERT INTO tariff (id, code, days, price_uzs, sort_order, active) VALUES ($1, $2, 30, 59900, 1, true)`,
		tariffID, tariffCode); err != nil {
		t.Fatalf("seed tariff: %v", err)
	}

	paymentID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key)
		 VALUES ($1, $2, $3, 59900, 'payme', $4, $5)`,
		paymentID, profileID, tariffID, status, uuid.New().String()); err != nil {
		t.Fatalf("seed payment: %v", err)
	}

	return paymentID
}

// seedPaymentWithTariffDays is seedPayment with a caller-chosen tariff.days,
// for TestPerformTransaction_GrantDaysFailureRollsBack: a huge days value
// overflows the int64-nanosecond time.Duration arithmetic in
// billing.Service.GrantDays (days * 24 * time.Hour wraps around to a
// negative duration), so the computed entitlement end lands before its
// start — deterministically tripping entitlement's `CHECK (ends_at >
// starts_at)` constraint on insert, without needing any FK trickery.
func seedPaymentWithTariffDays(t *testing.T, pool *pgxpool.Pool, status string, days int32) uuid.UUID {
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
		`INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key)
		 VALUES ($1, $2, $3, 59900, 'payme', $4, $5)`,
		paymentID, profileID, tariffID, status, uuid.New().String()); err != nil {
		t.Fatalf("seed payment: %v", err)
	}

	return paymentID
}

// rpcCall drives h.ServeHTTP with a valid Basic-auth JSON-RPC request and
// decodes the raw response body.
func rpcCall(t *testing.T, h *Handler, method string, params any) map[string]json.RawMessage {
	t.Helper()

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	body := fmt.Sprintf(`{"method":%q,"params":%s,"id":1}`, method, paramsJSON)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/payme", strings.NewReader(body))
	req.Header.Set("Authorization", basicAuthHeader("Paycom", methodsTestKey))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	var resp map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
	}
	return resp
}

func rpcErrorCode(t *testing.T, resp map[string]json.RawMessage) (int, bool) {
	t.Helper()
	raw, ok := resp["error"]
	if !ok {
		return 0, false
	}
	var e struct {
		Code int    `json:"code"`
		Data string `json:"data"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("decode error object: %v", err)
	}
	return e.Code, true
}

// --- CheckPerformTransaction ---------------------------------------------

func TestCheckPerformTransaction_Success(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "created")

	resp := rpcCall(t, h, "CheckPerformTransaction", map[string]any{
		"amount":  5990000,
		"account": map[string]string{"order_id": paymentID.String()},
	})

	if code, isErr := rpcErrorCode(t, resp); isErr {
		t.Fatalf("unexpected error, code=%d", code)
	}
	var result struct {
		Allow bool `json:"allow"`
	}
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("decode result: %v (resp=%v)", err, resp)
	}
	if !result.Allow {
		t.Errorf("allow = false, want true")
	}
}

func TestCheckPerformTransaction_WrongAmount(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "created")

	resp := rpcCall(t, h, "CheckPerformTransaction", map[string]any{
		"amount":  1000000,
		"account": map[string]string{"order_id": paymentID.String()},
	})

	code, isErr := rpcErrorCode(t, resp)
	if !isErr {
		t.Fatalf("expected error, got result=%s", resp["result"])
	}
	if code != -31001 {
		t.Errorf("code = %d, want -31001", code)
	}
}

func TestCheckPerformTransaction_UnknownAccount(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	_ = seedPayment(t, pool, "created")

	resp := rpcCall(t, h, "CheckPerformTransaction", map[string]any{
		"amount":  5990000,
		"account": map[string]string{"order_id": uuid.New().String()},
	})

	code, isErr := rpcErrorCode(t, resp)
	if !isErr {
		t.Fatalf("expected error, got result=%s", resp["result"])
	}
	if code != -31050 {
		t.Errorf("code = %d, want -31050", code)
	}
	var e struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(resp["error"], &e); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if e.Data != "order_id" {
		t.Errorf("data = %q, want %q", e.Data, "order_id")
	}
}

func TestCheckPerformTransaction_NotPayableStatus(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "pending")

	resp := rpcCall(t, h, "CheckPerformTransaction", map[string]any{
		"amount":  5990000,
		"account": map[string]string{"order_id": paymentID.String()},
	})

	code, isErr := rpcErrorCode(t, resp)
	if !isErr {
		t.Fatalf("expected error, got result=%s", resp["result"])
	}
	if code != -31050 {
		t.Errorf("code = %d, want -31050", code)
	}
}

// --- CreateTransaction -----------------------------------------------------

func TestCreateTransaction_New(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "created")
	paymeID := "payme-txn-1"

	resp := rpcCall(t, h, "CreateTransaction", map[string]any{
		"id":      paymeID,
		"time":    1234567890000,
		"amount":  5990000,
		"account": map[string]string{"order_id": paymentID.String()},
	})

	if code, isErr := rpcErrorCode(t, resp); isErr {
		t.Fatalf("unexpected error, code=%d", code)
	}
	var result struct {
		CreateTime  int64  `json:"create_time"`
		Transaction string `json:"transaction"`
		State       int32  `json:"state"`
	}
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("decode result: %v (resp=%v)", err, resp)
	}
	if result.Transaction != paymentID.String() {
		t.Errorf("transaction = %q, want %q", result.Transaction, paymentID.String())
	}
	if result.State != 1 {
		t.Errorf("state = %d, want 1", result.State)
	}
	if result.CreateTime == 0 {
		t.Errorf("create_time = 0, want nonzero")
	}

	// payment.status moved to 'pending'
	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM payment WHERE id = $1`, paymentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Errorf("payment.status = %q, want pending", status)
	}

	// payme_transaction row exists with state=1
	var state int32
	if err := pool.QueryRow(context.Background(),
		`SELECT state FROM payme_transaction WHERE payme_id = $1`, paymeID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != 1 {
		t.Errorf("payme_transaction.state = %d, want 1", state)
	}
}

func TestCreateTransaction_Idempotent(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "created")
	paymeID := "payme-txn-2"

	params := map[string]any{
		"id":      paymeID,
		"time":    1234567890000,
		"amount":  5990000,
		"account": map[string]string{"order_id": paymentID.String()},
	}

	first := rpcCall(t, h, "CreateTransaction", params)
	var firstResult struct {
		CreateTime  int64  `json:"create_time"`
		Transaction string `json:"transaction"`
		State       int32  `json:"state"`
	}
	if err := json.Unmarshal(first["result"], &firstResult); err != nil {
		t.Fatalf("decode first result: %v (resp=%v)", err, first)
	}

	second := rpcCall(t, h, "CreateTransaction", params)
	if code, isErr := rpcErrorCode(t, second); isErr {
		t.Fatalf("unexpected error on replay, code=%d", code)
	}
	var secondResult struct {
		CreateTime  int64  `json:"create_time"`
		Transaction string `json:"transaction"`
		State       int32  `json:"state"`
	}
	if err := json.Unmarshal(second["result"], &secondResult); err != nil {
		t.Fatalf("decode second result: %v (resp=%v)", err, second)
	}

	if secondResult != firstResult {
		t.Errorf("replay result = %+v, want same as first %+v", secondResult, firstResult)
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM payme_transaction WHERE payme_id = $1`, paymeID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("payme_transaction row count = %d, want 1 (no duplicate insert)", count)
	}
}

func TestCreateTransaction_ConflictActiveTransaction(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "created")

	// First payme_id claims the payment.
	first := rpcCall(t, h, "CreateTransaction", map[string]any{
		"id":      "payme-txn-a",
		"time":    1234567890000,
		"amount":  5990000,
		"account": map[string]string{"order_id": paymentID.String()},
	})
	if code, isErr := rpcErrorCode(t, first); isErr {
		t.Fatalf("unexpected error on first create, code=%d", code)
	}

	// A different payme_id for the same payment must conflict: the
	// payment now has an active (state 1) transaction already.
	second := rpcCall(t, h, "CreateTransaction", map[string]any{
		"id":      "payme-txn-b",
		"time":    1234567890111,
		"amount":  5990000,
		"account": map[string]string{"order_id": paymentID.String()},
	})
	code, isErr := rpcErrorCode(t, second)
	if !isErr {
		t.Fatalf("expected error, got result=%s", second["result"])
	}
	if code != -31008 {
		t.Errorf("code = %d, want -31008", code)
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM payme_transaction WHERE payme_id = 'payme-txn-b'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("payme-txn-b row count = %d, want 0 (conflicting create must not insert)", count)
	}
}

// --- PerformTransaction ---------------------------------------------------

type performResult struct {
	Transaction string `json:"transaction"`
	PerformTime int64  `json:"perform_time"`
	State       int32  `json:"state"`
}

func TestPerformTransaction_Success(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "pending")
	paymeID := "payme-perform-1"
	seedPaymeTransaction(t, pool, paymeID, paymentID, 1, time.Now().UnixMilli())
	profileID := profileOf(t, pool, paymentID)

	resp := rpcCall(t, h, "PerformTransaction", map[string]any{"id": paymeID})

	if code, isErr := rpcErrorCode(t, resp); isErr {
		t.Fatalf("unexpected error, code=%d", code)
	}
	var result performResult
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("decode result: %v (resp=%v)", err, resp)
	}
	if result.Transaction != paymentID.String() {
		t.Errorf("transaction = %q, want %q", result.Transaction, paymentID.String())
	}
	if result.State != 2 {
		t.Errorf("state = %d, want 2", result.State)
	}
	if result.PerformTime == 0 {
		t.Errorf("perform_time = 0, want nonzero")
	}

	var status string
	var paidAt *time.Time
	if err := pool.QueryRow(context.Background(),
		`SELECT status, paid_at FROM payment WHERE id = $1`, paymentID).Scan(&status, &paidAt); err != nil {
		t.Fatal(err)
	}
	if status != "paid" {
		t.Errorf("payment.status = %q, want paid", status)
	}
	if paidAt == nil {
		t.Errorf("payment.paid_at = nil, want set")
	}

	active, until, err := h.Svc.Status(context.Background(), profileID)
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

func TestPerformTransaction_Idempotent(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "pending")
	paymeID := "payme-perform-2"
	seedPaymeTransaction(t, pool, paymeID, paymentID, 1, time.Now().UnixMilli())
	profileID := profileOf(t, pool, paymentID)

	first := rpcCall(t, h, "PerformTransaction", map[string]any{"id": paymeID})
	var firstResult performResult
	if err := json.Unmarshal(first["result"], &firstResult); err != nil {
		t.Fatalf("decode first result: %v (resp=%v)", err, first)
	}
	_, until1, err := h.Svc.Status(context.Background(), profileID)
	if err != nil {
		t.Fatalf("Status (1): %v", err)
	}

	second := rpcCall(t, h, "PerformTransaction", map[string]any{"id": paymeID})
	if code, isErr := rpcErrorCode(t, second); isErr {
		t.Fatalf("unexpected error on replay, code=%d", code)
	}
	var secondResult performResult
	if err := json.Unmarshal(second["result"], &secondResult); err != nil {
		t.Fatalf("decode second result: %v (resp=%v)", err, second)
	}
	if secondResult != firstResult {
		t.Errorf("replay result = %+v, want same as first %+v", secondResult, firstResult)
	}

	_, until2, err := h.Svc.Status(context.Background(), profileID)
	if err != nil {
		t.Fatalf("Status (2): %v", err)
	}
	if until1 == nil || until2 == nil || !until1.Equal(*until2) {
		t.Errorf("entitlement end changed on replay: %v -> %v, want unchanged (no double grant)", until1, until2)
	}
}

func TestPerformTransaction_NotFound(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)

	resp := rpcCall(t, h, "PerformTransaction", map[string]any{"id": "no-such-payme-id"})

	code, isErr := rpcErrorCode(t, resp)
	if !isErr {
		t.Fatalf("expected error, got result=%s", resp["result"])
	}
	if code != -31003 {
		t.Errorf("code = %d, want -31003", code)
	}
}

func TestPerformTransaction_ExpiredPending(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "pending")
	paymeID := "payme-perform-expired"
	oldCreateTime := time.Now().Add(-13 * time.Hour).UnixMilli()
	seedPaymeTransaction(t, pool, paymeID, paymentID, 1, oldCreateTime)

	resp := rpcCall(t, h, "PerformTransaction", map[string]any{"id": paymeID})

	code, isErr := rpcErrorCode(t, resp)
	if !isErr {
		t.Fatalf("expected error, got result=%s", resp["result"])
	}
	if code != -31008 {
		t.Errorf("code = %d, want -31008", code)
	}

	var state int32
	var reason int32
	if err := pool.QueryRow(context.Background(),
		`SELECT state, reason FROM payme_transaction WHERE payme_id = $1`, paymeID).Scan(&state, &reason); err != nil {
		t.Fatal(err)
	}
	if state != -1 {
		t.Errorf("payme_transaction.state = %d, want -1", state)
	}
	if reason != 4 {
		t.Errorf("payme_transaction.reason = %d, want 4", reason)
	}

	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM payment WHERE id = $1`, paymentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "canceled" {
		t.Errorf("payment.status = %q, want canceled", status)
	}
}

// TestPerformTransaction_GrantDaysFailureRollsBack proves the review-round-1
// fix for the "non-atomic multi-step write can strand a paid customer"
// finding: if GrantDays fails after the state flip would otherwise have
// happened, the whole performTransaction DB transaction must roll back, so
// payme_transaction.state stays 1 (not 2) and payment.status stays
// 'pending' (not 'paid') — leaving room for a genuine retry to actually
// retry, instead of the state==2 idempotent-replay branch silently
// fabricating "success" forever with no entitlement ever granted.
//
// The GrantDays failure is forced deterministically via tariff.days
// overflowing GrantDays' internal time.Duration arithmetic (see
// seedPaymentWithTariffDays) rather than via any FK trick, since
// payment.profile_id's FK to profile is NOT NULL and enforced immediately
// on insert — a payment row pointing at a nonexistent profile can't be
// seeded in the first place.
func TestPerformTransaction_GrantDaysFailureRollsBack(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPaymentWithTariffDays(t, pool, "pending", 2000000000)
	paymeID := "payme-perform-grantfail"
	seedPaymeTransaction(t, pool, paymeID, paymentID, 1, time.Now().UnixMilli())

	resp := rpcCall(t, h, "PerformTransaction", map[string]any{"id": paymeID})

	if code, isErr := rpcErrorCode(t, resp); !isErr {
		t.Fatalf("expected error (GrantDays should have failed), got result=%s", resp["result"])
	} else if code == 0 {
		t.Errorf("error code = 0, want nonzero")
	}

	var state int32
	if err := pool.QueryRow(context.Background(),
		`SELECT state FROM payme_transaction WHERE payme_id = $1`, paymeID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != 1 {
		t.Errorf("payme_transaction.state = %d, want 1 (rolled back, not stranded at 2)", state)
	}

	var status string
	var paidAt *time.Time
	if err := pool.QueryRow(context.Background(),
		`SELECT status, paid_at FROM payment WHERE id = $1`, paymentID).Scan(&status, &paidAt); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Errorf("payment.status = %q, want pending (rolled back, not paid)", status)
	}
	if paidAt != nil {
		t.Errorf("payment.paid_at = %v, want nil (rolled back)", *paidAt)
	}

	// A genuine retry with the same payme_id must still be able to try
	// again (not silently replay a fabricated "success" via the state==2
	// idempotent branch) — it'll hit the same deterministic GrantDays
	// failure again here, but the important thing is it's NOT treated as
	// an idempotent replay of a prior success.
	retry := rpcCall(t, h, "PerformTransaction", map[string]any{"id": paymeID})
	if _, isErr := rpcErrorCode(t, retry); !isErr {
		t.Fatalf("retry unexpectedly succeeded with result=%s (should still fail deterministically, proving it re-attempted rather than idempotent-replayed)", retry["result"])
	}
}

// --- CancelTransaction -----------------------------------------------------

type cancelResult struct {
	Transaction string `json:"transaction"`
	CancelTime  int64  `json:"cancel_time"`
	State       int32  `json:"state"`
}

func TestCancelTransaction_FromPending(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "pending")
	paymeID := "payme-cancel-1"
	seedPaymeTransaction(t, pool, paymeID, paymentID, 1, time.Now().UnixMilli())

	resp := rpcCall(t, h, "CancelTransaction", map[string]any{"id": paymeID, "reason": 5})

	if code, isErr := rpcErrorCode(t, resp); isErr {
		t.Fatalf("unexpected error, code=%d", code)
	}
	var result cancelResult
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("decode result: %v (resp=%v)", err, resp)
	}
	if result.Transaction != paymentID.String() {
		t.Errorf("transaction = %q, want %q", result.Transaction, paymentID.String())
	}
	if result.State != -1 {
		t.Errorf("state = %d, want -1", result.State)
	}
	if result.CancelTime == 0 {
		t.Errorf("cancel_time = 0, want nonzero")
	}

	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM payment WHERE id = $1`, paymentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "canceled" {
		t.Errorf("payment.status = %q, want canceled", status)
	}
}

func TestCancelTransaction_FromPaid(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "paid")
	paymeID := "payme-cancel-2"
	seedPaymeTransaction(t, pool, paymeID, paymentID, 2, time.Now().UnixMilli())

	resp := rpcCall(t, h, "CancelTransaction", map[string]any{"id": paymeID, "reason": 3})

	if code, isErr := rpcErrorCode(t, resp); isErr {
		t.Fatalf("unexpected error, code=%d", code)
	}
	var result cancelResult
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("decode result: %v (resp=%v)", err, resp)
	}
	if result.State != -2 {
		t.Errorf("state = %d, want -2", result.State)
	}

	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM payment WHERE id = $1`, paymentID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "refunded" {
		t.Errorf("payment.status = %q, want refunded", status)
	}
}

func TestCancelTransaction_Idempotent(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "pending")
	paymeID := "payme-cancel-3"
	seedPaymeTransaction(t, pool, paymeID, paymentID, 1, time.Now().UnixMilli())

	first := rpcCall(t, h, "CancelTransaction", map[string]any{"id": paymeID, "reason": 5})
	var firstResult cancelResult
	if err := json.Unmarshal(first["result"], &firstResult); err != nil {
		t.Fatalf("decode first result: %v (resp=%v)", err, first)
	}

	second := rpcCall(t, h, "CancelTransaction", map[string]any{"id": paymeID, "reason": 5})
	if code, isErr := rpcErrorCode(t, second); isErr {
		t.Fatalf("unexpected error on replay, code=%d", code)
	}
	var secondResult cancelResult
	if err := json.Unmarshal(second["result"], &secondResult); err != nil {
		t.Fatalf("decode second result: %v (resp=%v)", err, second)
	}
	if secondResult != firstResult {
		t.Errorf("replay result = %+v, want same as first %+v", secondResult, firstResult)
	}
}

func TestCancelTransaction_NotFound(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)

	resp := rpcCall(t, h, "CancelTransaction", map[string]any{"id": "no-such-payme-id", "reason": 1})

	code, isErr := rpcErrorCode(t, resp)
	if !isErr {
		t.Fatalf("expected error, got result=%s", resp["result"])
	}
	if code != -31003 {
		t.Errorf("code = %d, want -31003", code)
	}
}

// --- CheckTransaction -------------------------------------------------------

func TestCheckTransaction_Pending(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "pending")
	paymeID := "payme-check-1"
	createTime := time.Now().UnixMilli()
	seedPaymeTransaction(t, pool, paymeID, paymentID, 1, createTime)

	resp := rpcCall(t, h, "CheckTransaction", map[string]any{"id": paymeID})

	if code, isErr := rpcErrorCode(t, resp); isErr {
		t.Fatalf("unexpected error, code=%d", code)
	}
	var result checkTransactionResult
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("decode result: %v (resp=%v)", err, resp)
	}
	if result.CreateTime != createTime {
		t.Errorf("create_time = %d, want %d", result.CreateTime, createTime)
	}
	if result.PerformTime != 0 {
		t.Errorf("perform_time = %d, want 0", result.PerformTime)
	}
	if result.CancelTime != 0 {
		t.Errorf("cancel_time = %d, want 0", result.CancelTime)
	}
	if result.Transaction != paymentID.String() {
		t.Errorf("transaction = %q, want %q", result.Transaction, paymentID.String())
	}
	if result.State != 1 {
		t.Errorf("state = %d, want 1", result.State)
	}
	if result.Reason != 0 {
		t.Errorf("reason = %d, want 0", result.Reason)
	}
}

func TestCheckTransaction_Performed(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "paid")
	paymeID := "payme-check-2"
	createTime := time.Now().Add(-time.Hour).UnixMilli()
	performTime := time.Now().UnixMilli()
	seedPaymeTransaction(t, pool, paymeID, paymentID, 1, createTime)
	if _, err := pool.Exec(context.Background(),
		`UPDATE payme_transaction SET state = 2, perform_time = $1 WHERE payme_id = $2`,
		performTime, paymeID); err != nil {
		t.Fatalf("update payme_transaction: %v", err)
	}

	resp := rpcCall(t, h, "CheckTransaction", map[string]any{"id": paymeID})

	if code, isErr := rpcErrorCode(t, resp); isErr {
		t.Fatalf("unexpected error, code=%d", code)
	}
	var result checkTransactionResult
	if err := json.Unmarshal(resp["result"], &result); err != nil {
		t.Fatalf("decode result: %v (resp=%v)", err, resp)
	}
	if result.CreateTime != createTime {
		t.Errorf("create_time = %d, want %d", result.CreateTime, createTime)
	}
	if result.PerformTime != performTime {
		t.Errorf("perform_time = %d, want %d", result.PerformTime, performTime)
	}
	if result.State != 2 {
		t.Errorf("state = %d, want 2", result.State)
	}
}

func TestCheckTransaction_NotFound(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)

	resp := rpcCall(t, h, "CheckTransaction", map[string]any{"id": "no-such-payme-id"})

	code, isErr := rpcErrorCode(t, resp)
	if !isErr {
		t.Fatalf("expected error, got result=%s", resp["result"])
	}
	if code != -31003 {
		t.Errorf("code = %d, want -31003", code)
	}
}

// --- GetStatement ------------------------------------------------------------

func TestGetStatement_Range(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	payment1 := seedPayment(t, pool, "pending")
	payment2 := seedPaymentTariffCode(t, pool, "paid", "gentra-stmt-2")
	base := time.Now().UnixMilli()
	seedPaymeTransaction(t, pool, "payme-stmt-1", payment1, 1, base)
	seedPaymeTransaction(t, pool, "payme-stmt-2", payment2, 2, base+1000)

	resp := rpcCall(t, h, "GetStatement", map[string]any{"from": base - 1, "to": base + 2000})

	if code, isErr := rpcErrorCode(t, resp); isErr {
		t.Fatalf("unexpected error, code=%d", code)
	}
	var entries []statementEntry
	if err := json.Unmarshal(resp["result"], &entries); err != nil {
		t.Fatalf("decode result: %v (resp=%v)", err, resp)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].ID != "payme-stmt-1" || entries[1].ID != "payme-stmt-2" {
		t.Errorf("order = [%s, %s], want ascending by create_time", entries[0].ID, entries[1].ID)
	}
	if entries[0].Account.OrderID != payment1.String() {
		t.Errorf("account.order_id = %q, want %q", entries[0].Account.OrderID, payment1.String())
	}
	if entries[0].Amount != 5990000 {
		t.Errorf("amount = %d, want 5990000", entries[0].Amount)
	}
	if entries[0].CreateTime != base {
		t.Errorf("create_time = %d, want %d", entries[0].CreateTime, base)
	}
	if entries[0].Time != base {
		t.Errorf("time = %d, want %d", entries[0].Time, base)
	}
	if entries[0].Transaction != payment1.String() {
		t.Errorf("transaction = %q, want %q", entries[0].Transaction, payment1.String())
	}
	if entries[1].State != 2 {
		t.Errorf("entries[1].state = %d, want 2", entries[1].State)
	}
}

func TestGetStatement_EmptyRange(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "pending")
	base := time.Now().UnixMilli()
	seedPaymeTransaction(t, pool, "payme-stmt-empty", paymentID, 1, base)

	resp := rpcCall(t, h, "GetStatement", map[string]any{"from": base + 100000, "to": base + 200000})

	if code, isErr := rpcErrorCode(t, resp); isErr {
		t.Fatalf("unexpected error, code=%d", code)
	}
	if string(resp["result"]) != "[]" {
		t.Errorf("result = %s, want []", resp["result"])
	}
}
