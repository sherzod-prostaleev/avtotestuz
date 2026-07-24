package payme

import (
	"context"
	"encoding/json"
	"testing"

	"avtotest.uz/backend/internal/testdb"
)

// TestFullFlow_PerformGrantsEntitlement drives CheckPerformTransaction →
// CreateTransaction → PerformTransaction against a single seeded payment
// through Handler.ServeHTTP (real JSON-RPC bodies, real Basic-auth), the
// same path Payme itself would call. This is the end-to-end proof that
// Tasks 3-6's pieces are wired together correctly, not a re-test of any
// single method in isolation.
func TestFullFlow_PerformGrantsEntitlement(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPayment(t, pool, "created")
	profileID := profileOf(t, pool, paymentID)
	paymeID := "payme-flow-perform"

	checkResp := rpcCall(t, h, "CheckPerformTransaction", map[string]any{
		"amount":  5990000,
		"account": map[string]string{"order_id": paymentID.String()},
	})
	if code, isErr := rpcErrorCode(t, checkResp); isErr {
		t.Fatalf("CheckPerformTransaction: unexpected error, code=%d", code)
	}
	var checkResult struct {
		Allow bool `json:"allow"`
	}
	if err := json.Unmarshal(checkResp["result"], &checkResult); err != nil {
		t.Fatalf("decode CheckPerformTransaction result: %v", err)
	}
	if !checkResult.Allow {
		t.Fatalf("CheckPerformTransaction allow = false, want true")
	}

	createResp := rpcCall(t, h, "CreateTransaction", map[string]any{
		"id":      paymeID,
		"time":    1234567890000,
		"amount":  5990000,
		"account": map[string]string{"order_id": paymentID.String()},
	})
	if code, isErr := rpcErrorCode(t, createResp); isErr {
		t.Fatalf("CreateTransaction: unexpected error, code=%d", code)
	}
	var createResult struct {
		State int32 `json:"state"`
	}
	if err := json.Unmarshal(createResp["result"], &createResult); err != nil {
		t.Fatalf("decode CreateTransaction result: %v", err)
	}
	if createResult.State != 1 {
		t.Fatalf("CreateTransaction state = %d, want 1", createResult.State)
	}

	performResp := rpcCall(t, h, "PerformTransaction", map[string]any{"id": paymeID})
	if code, isErr := rpcErrorCode(t, performResp); isErr {
		t.Fatalf("PerformTransaction: unexpected error, code=%d", code)
	}
	var performResultDecoded performResult
	if err := json.Unmarshal(performResp["result"], &performResultDecoded); err != nil {
		t.Fatalf("decode PerformTransaction result: %v", err)
	}
	if performResultDecoded.State != 2 {
		t.Fatalf("PerformTransaction state = %d, want 2", performResultDecoded.State)
	}

	var status string
	var paidAt *string
	if err := pool.QueryRow(context.Background(),
		`SELECT status, paid_at::text FROM payment WHERE id = $1`, paymentID).Scan(&status, &paidAt); err != nil {
		t.Fatalf("query payment: %v", err)
	}
	if status != "paid" {
		t.Errorf("payment.status = %q, want paid", status)
	}
	if paidAt == nil {
		t.Errorf("payment.paid_at = nil, want set")
	}

	active, until, err := h.Svc.Status(context.Background(), profileID)
	if err != nil {
		t.Fatalf("billing.Service.Status: %v", err)
	}
	if !active {
		t.Errorf("entitlement active = false, want true")
	}
	if until == nil {
		t.Errorf("entitlement until = nil, want set")
	}
}

// TestFullFlow_CancelFromPending drives CheckPerformTransaction →
// CreateTransaction → CancelTransaction against a second, independent
// payment, proving the cancel path (state 1 → -1) works end to end and
// lands the payment in 'canceled' (Task 5's state 1→cancel mapping).
func TestFullFlow_CancelFromPending(t *testing.T) {
	pool := testdb.New(t)
	h := newMethodsHandler(pool)
	paymentID := seedPaymentTariffCode(t, pool, "created", "gentra-flow-cancel")
	paymeID := "payme-flow-cancel"

	checkResp := rpcCall(t, h, "CheckPerformTransaction", map[string]any{
		"amount":  5990000,
		"account": map[string]string{"order_id": paymentID.String()},
	})
	if code, isErr := rpcErrorCode(t, checkResp); isErr {
		t.Fatalf("CheckPerformTransaction: unexpected error, code=%d", code)
	}
	var checkResult struct {
		Allow bool `json:"allow"`
	}
	if err := json.Unmarshal(checkResp["result"], &checkResult); err != nil {
		t.Fatalf("decode CheckPerformTransaction result: %v", err)
	}
	if !checkResult.Allow {
		t.Fatalf("CheckPerformTransaction allow = false, want true")
	}

	createResp := rpcCall(t, h, "CreateTransaction", map[string]any{
		"id":      paymeID,
		"time":    1234567890000,
		"amount":  5990000,
		"account": map[string]string{"order_id": paymentID.String()},
	})
	if code, isErr := rpcErrorCode(t, createResp); isErr {
		t.Fatalf("CreateTransaction: unexpected error, code=%d", code)
	}
	var createResult struct {
		State int32 `json:"state"`
	}
	if err := json.Unmarshal(createResp["result"], &createResult); err != nil {
		t.Fatalf("decode CreateTransaction result: %v", err)
	}
	if createResult.State != 1 {
		t.Fatalf("CreateTransaction state = %d, want 1", createResult.State)
	}

	cancelResp := rpcCall(t, h, "CancelTransaction", map[string]any{"id": paymeID, "reason": 5})
	if code, isErr := rpcErrorCode(t, cancelResp); isErr {
		t.Fatalf("CancelTransaction: unexpected error, code=%d", code)
	}
	var cancelResultDecoded cancelResult
	if err := json.Unmarshal(cancelResp["result"], &cancelResultDecoded); err != nil {
		t.Fatalf("decode CancelTransaction result: %v", err)
	}
	if cancelResultDecoded.State != -1 {
		t.Fatalf("CancelTransaction state = %d, want -1", cancelResultDecoded.State)
	}

	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM payment WHERE id = $1`, paymentID).Scan(&status); err != nil {
		t.Fatalf("query payment: %v", err)
	}
	if status != "canceled" {
		t.Errorf("payment.status = %q, want canceled", status)
	}
}
