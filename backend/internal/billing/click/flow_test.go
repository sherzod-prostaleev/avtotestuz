package click

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"avtotest.uz/backend/internal/testdb"
)

// TestFlow_PrepareThenComplete_Success drives a full Prepare→Complete happy
// path through Handler.ServeHTTP for a single seeded payment — the first
// test to exercise both RPCs together (Tasks 4/5 each tested their own RPC
// in isolation, seeding a click_transaction row directly for Complete
// instead of obtaining it from a real Prepare response). Asserts
// payment.status='paid' and billing.Service.Status reports active=true,
// proving the merchant_prepare_id handed back by Prepare is exactly what
// Complete needs to resolve the same transaction end to end.
func TestFlow_PrepareThenComplete_Success(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "created")

	prep := clickCall(t, h, prepareRequest("click-flow-1", paymentID.String(), "59900"))
	if prep.Error != errSuccess {
		t.Fatalf("prepare error = %d, want %d (note=%q)", prep.Error, errSuccess, prep.ErrorNote)
	}
	txID, err := uuid.Parse(prep.MerchantPrepareID)
	if err != nil {
		t.Fatalf("merchant_prepare_id = %q, not a valid uuid: %v", prep.MerchantPrepareID, err)
	}

	comp := clickCall(t, h, completeRequest("click-flow-1", txID, paymentID, "59900"))
	if comp.Error != errSuccess {
		t.Fatalf("complete error = %d, want %d (note=%q)", comp.Error, errSuccess, comp.ErrorNote)
	}

	ctx := context.Background()
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM payment WHERE id = $1`, paymentID).Scan(&status); err != nil {
		t.Fatalf("query payment: %v", err)
	}
	if status != "paid" {
		t.Errorf("payment.status = %q, want paid", status)
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

// TestFlow_PrepareThenComplete_ClickDeclines drives Prepare then a
// Click-declined Complete (error="-1") through Handler.ServeHTTP for a
// second seeded payment, asserting payment.status='canceled' and
// click_transaction.state=-1 — proving rejectTransaction correctly resolves
// and rejects a transaction that was itself created by a real Prepare call,
// not just a directly-seeded row.
func TestFlow_PrepareThenComplete_ClickDeclines(t *testing.T) {
	pool := testdb.New(t)
	h := newClickHandler(pool)
	paymentID := seedClickPayment(t, pool, "created")

	prep := clickCall(t, h, prepareRequest("click-flow-2", paymentID.String(), "59900"))
	if prep.Error != errSuccess {
		t.Fatalf("prepare error = %d, want %d (note=%q)", prep.Error, errSuccess, prep.ErrorNote)
	}
	txID, err := uuid.Parse(prep.MerchantPrepareID)
	if err != nil {
		t.Fatalf("merchant_prepare_id = %q, not a valid uuid: %v", prep.MerchantPrepareID, err)
	}

	req := completeRequest("click-flow-2", txID, paymentID, "59900")
	req.Error = "-1"
	req.ErrorNote = "user declined"
	comp := clickCall(t, h, req)
	if comp.Error != errCancelled {
		t.Fatalf("complete error = %d, want %d (note=%q)", comp.Error, errCancelled, comp.ErrorNote)
	}

	ctx := context.Background()
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM payment WHERE id = $1`, paymentID).Scan(&status); err != nil {
		t.Fatalf("query payment: %v", err)
	}
	if status != "canceled" {
		t.Errorf("payment.status = %q, want canceled", status)
	}

	var state int32
	if err := pool.QueryRow(ctx,
		`SELECT state FROM click_transaction WHERE id = $1`, txID).Scan(&state); err != nil {
		t.Fatalf("query click_transaction: %v", err)
	}
	if state != -1 {
		t.Errorf("click_transaction.state = %d, want -1", state)
	}
}
