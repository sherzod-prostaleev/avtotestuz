package payme

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/db/sqlc"
)

// accountParams is the `account` object Payme sends on every
// account-bearing method: `{"account":{"order_id":"<payment id>"}}`.
type accountParams struct {
	OrderID string `json:"order_id"`
}

// checkPerformParams is CheckPerformTransaction's `params`.
type checkPerformParams struct {
	Amount  int64         `json:"amount"`
	Account accountParams `json:"account"`
}

// checkPerformResult is the successful CheckPerformTransaction result.
type checkPerformResult struct {
	Allow bool `json:"allow"`
}

// createTransactionParams is CreateTransaction's `params`. ID is Payme's
// own transaction id (payme_transaction.payme_id); Time is Payme's request
// timestamp (ms) — distinct from our own create_time, which we set from
// the server clock per the spec.
type createTransactionParams struct {
	ID      string        `json:"id"`
	Time    int64         `json:"time"`
	Amount  int64         `json:"amount"`
	Account accountParams `json:"account"`
}

// createTransactionResult is CreateTransaction's result, both for a
// freshly-created transaction and for the idempotent replay of an existing
// one.
type createTransactionResult struct {
	CreateTime  int64  `json:"create_time"`
	Transaction string `json:"transaction"`
	State       int32  `json:"state"`
}

// checkPerform validates account.order_id + amount for CheckPerformTransaction:
// the payment must exist, be in the payable 'created' status, and its
// amount_uzs*100 must equal params.amount (tiyin).
func (h *Handler) checkPerform(ctx context.Context, p checkPerformParams) (checkPerformResult, *rpcError) {
	if _, rpcErr := h.validateAccountAmount(ctx, p.Account.OrderID, p.Amount, false); rpcErr != nil {
		return checkPerformResult{}, rpcErr
	}
	return checkPerformResult{Allow: true}, nil
}

// createTransaction implements CreateTransaction. If a payme_transaction
// with this exact payme_id already exists, it's a replay: return the
// existing create_time/state without touching account/amount or inserting
// a duplicate row. For a genuinely new payme_id, re-validate account +
// amount, reject if the payment already has another active (state 1/2)
// transaction, then insert the new transaction and move payment to
// 'pending'.
func (h *Handler) createTransaction(ctx context.Context, p createTransactionParams) (createTransactionResult, *rpcError) {
	existing, err := h.Q.GetPaymeTransaction(ctx, p.ID)
	if err == nil {
		return createTransactionResult{
			CreateTime:  existing.CreateTime,
			Transaction: existing.PaymentID.String(),
			State:       existing.State,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return createTransactionResult{}, errInternal
	}

	// allowPending=true: by the time a second, genuinely-new payme_id
	// reaches this re-validation, a prior CreateTransaction call may
	// already have flipped payment.status to 'pending' (that's exactly
	// the case we want to fall through to the active-transaction check
	// below, which reports the real conflict as -31008 rather than
	// misreporting it as "account not payable").
	payment, rpcErr := h.validateAccountAmount(ctx, p.Account.OrderID, p.Amount, true)
	if rpcErr != nil {
		return createTransactionResult{}, rpcErr
	}

	if _, err := h.Q.GetActivePaymeTxByPayment(ctx, payment.ID); err == nil {
		// Another transaction (state 1 or 2) is already active for this
		// payment — a second payme_id can't also claim it.
		return createTransactionResult{}, errTransactionState
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return createTransactionResult{}, errInternal
	}

	createTime := time.Now().UnixMilli()
	if err := h.Q.CreatePaymeTransaction(ctx, sqlc.CreatePaymeTransactionParams{
		PaymeID:     p.ID,
		PaymentID:   payment.ID,
		AmountTiyin: p.Amount,
		CreateTime:  createTime,
	}); err != nil {
		return createTransactionResult{}, errInternal
	}

	if err := h.Q.SetPaymentStatus(ctx, sqlc.SetPaymentStatusParams{
		ID:     payment.ID,
		Status: "pending",
	}); err != nil {
		return createTransactionResult{}, errInternal
	}

	return createTransactionResult{
		CreateTime:  createTime,
		Transaction: payment.ID.String(),
		State:       1,
	}, nil
}

// validateAccountAmount is the account+amount check shared by
// CheckPerformTransaction and CreateTransaction's new-id path: order_id
// must resolve to a payment in a payable status, and amount (tiyin) must
// equal payment.amount_uzs*100.
//
// allowPending distinguishes the two callers' notion of "payable":
// CheckPerformTransaction (allowPending=false) only allows the pristine
// 'created' status — an order already claimed by a transaction is not
// re-payable. CreateTransaction's new-payme_id re-validation
// (allowPending=true) must also accept 'pending', because a prior
// CreateTransaction call already moved the order there; that combination
// (status='pending' + an active payme_transaction) is exactly the -31008
// conflict CreateTransaction needs to detect downstream — treating it as
// -31050 here would hide the real conflict. Any other status ('paid',
// 'failed', 'canceled', 'refunded') is never payable, for either caller.
func (h *Handler) validateAccountAmount(ctx context.Context, orderID string, amountTiyin int64, allowPending bool) (sqlc.GetPaymentForPaymeRow, *rpcError) {
	id, err := uuid.Parse(orderID)
	if err != nil {
		return sqlc.GetPaymentForPaymeRow{}, errAccountNotFound
	}

	payment, err := h.Q.GetPaymentForPayme(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetPaymentForPaymeRow{}, errAccountNotFound
		}
		return sqlc.GetPaymentForPaymeRow{}, errInternal
	}

	payable := payment.Status == "created" || (allowPending && payment.Status == "pending")
	if !payable {
		return sqlc.GetPaymentForPaymeRow{}, errAccountNotFound
	}

	if amountTiyin != payment.AmountUzs*100 {
		return sqlc.GetPaymentForPaymeRow{}, errAmount
	}

	return payment, nil
}
