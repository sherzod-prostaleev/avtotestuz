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
		`INSERT INTO payment (id, profile_id, tariff_id, amount_uzs, provider, status, idempotency_key)
		 VALUES ($1, $2, $3, 59900, 'click', $4, $5)`,
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
