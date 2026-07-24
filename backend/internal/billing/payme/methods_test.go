package payme

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	return &Handler{Q: q, Svc: billing.Service{Q: q}, Key: methodsTestKey}
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
