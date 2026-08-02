package click

import (
	"context"
	"errors"
	"math"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
)

// prepare implements Click's Prepare (action=0): the merchant_trans_id must
// resolve to a payment in a payable status, and amount (so'm, unlike
// Payme's tiyin) must match payment.amount_uzs. Payable is a whitelist, not
// a blacklist of 'paid' — B3-2: blacklisting only 'paid' let any other
// terminal status ('failed', 'canceled', 'refunded', or migration 0047's
// admin-'voided') be silently re-prepared, flipped back to 'pending' and
// completed, resurrecting a dead or admin-voided order. The whitelist is
// 'created' (fresh) or 'pending' — 'pending' must stay payable because a
// prior Prepare already parked the order there, which is exactly the state
// the idempotent click_trans_id replay below (and a legitimate second
// Prepare racing a first) needs to observe; mirrors Payme's
// validateAccountAmount(allowPending=true). An already-'paid' payment keeps
// reporting -4 ("Already paid") rather than -5: that's Click's own
// reference behavior for re-preparing a completed order, distinct from a
// dead one. Every other non-payable status reports -5 (account not found),
// the same code Payme uses for any non-payable account. A click_trans_id
// already seen is an idempotent replay — return the existing
// merchant_prepare_id without touching state. Click itself signals a
// cancellation by sending a negative error code, which must be honored
// without creating a click_transaction row. Otherwise this inserts a new
// click_transaction (state=0, waiting) and moves the payment to 'pending'.
func (h *Handler) prepare(ctx context.Context, req clickRequest) clickResponse {
	paymentID, err := uuid.Parse(req.MerchantTransID)
	if err != nil {
		return errorResponse(req, errAccountNotFound)
	}
	payment, err := h.Q.GetPaymentForPayme(ctx, paymentID)
	if err != nil {
		return errorResponse(req, errAccountNotFound)
	}
	if payment.Status == "paid" {
		return errorResponse(req, errAlreadyPaid)
	}
	if payment.Status != "created" && payment.Status != "pending" {
		return errorResponse(req, errAccountNotFound)
	}

	amount, perr := strconv.ParseFloat(req.Amount, 64)
	if perr != nil || math.Abs(amount-float64(payment.AmountUzs)) > 0.01 {
		return errorResponse(req, errAmount)
	}

	if clickErr, _ := strconv.Atoi(req.Error); clickErr < 0 {
		return errorResponse(req, errCancelled)
	}

	if existing, err := h.Q.GetClickTransactionByClickTransID(ctx, req.ClickTransID); err == nil {
		return clickResponse{
			ClickTransID:      req.ClickTransID,
			MerchantTransID:   req.MerchantTransID,
			MerchantPrepareID: existing.ID.String(),
			Error:             errSuccess,
			ErrorNote:         errorNotes[errSuccess],
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return errorResponse(req, errBadRequest)
	}

	if _, err := h.Q.GetActiveClickTxByPayment(ctx, payment.ID); err == nil {
		return errorResponse(req, errBadRequest)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return errorResponse(req, errBadRequest)
	}

	txID, err := h.Q.CreateClickTransaction(ctx, sqlc.CreateClickTransactionParams{
		ClickTransID:  req.ClickTransID,
		ClickPaydocID: pgtype.Text{String: req.ClickPaydocID, Valid: req.ClickPaydocID != ""},
		PaymentID:     payment.ID,
		AmountUzs:     payment.AmountUzs,
	})
	if err != nil {
		return errorResponse(req, errBadRequest)
	}
	if err := h.Q.MarkPaymentPending(ctx, sqlc.MarkPaymentPendingParams{
		ID:            payment.ID,
		ProviderTxnID: pgtype.Text{String: req.ClickTransID, Valid: true},
	}); err != nil {
		return errorResponse(req, errBadRequest)
	}

	return clickResponse{
		ClickTransID:      req.ClickTransID,
		MerchantTransID:   req.MerchantTransID,
		MerchantPrepareID: txID.String(),
		Error:             errSuccess,
		ErrorNote:         errorNotes[errSuccess],
	}
}

// complete implements Click's Complete (action=1): merchant_prepare_id must
// resolve to a click_transaction created by a prior Prepare, and
// merchant_trans_id must match that transaction's own payment_id (Click
// echoes both ids back on Complete). An already-paid payment is an
// idempotent replay of a retried Complete — Click's own reference behavior
// for this case is -4 ("Already paid"), not a distinct success shape, and
// crucially this branch never re-runs GrantDays. A transaction already in
// state==-1 was already rejected and can't be completed again — checked
// before the payment-status whitelist below so a retried decline still
// reports -9 even if the payment has since moved on to 'voided' (rejected
// payments are voidable; see admin.Store.VoidPayment), not -5. Otherwise
// payment.status is whitelisted to 'pending' (B3-2, mirroring prepare's own
// whitelist fix): a click_transaction can only be legitimately completed
// while its payment is still the 'pending' one Prepare parked it in: any
// other status — including a payment that reached 'failed'/'canceled'/
// 'refunded'/admin-'voided' through some path other than this same
// transaction's own rejectTransaction — must not be confirmed, or a dead
// order could be granted entitlement out from under an admin's decision.
// That non-payable case reports -5, same as prepare's; the amount (so'm,
// unlike Payme's tiyin) is checked against the transaction's own amount_uzs
// (not payment.amount_uzs — the transaction is the authoritative record of
// what was actually prepared). Click's own error<0 on the Complete call
// means it is declining what it previously prepared: reject the
// transaction (transactionally) and report -9. Otherwise this is the real
// success path, delegated to confirmAndGrant for the atomic, row-locked
// confirm+grant sequence — see its doc comment for why that matters.
func (h *Handler) complete(ctx context.Context, req clickRequest) clickResponse {
	transactionID, err := uuid.Parse(req.MerchantPrepareID)
	if err != nil {
		return errorResponse(req, errTransactionNotFound)
	}
	tx, err := h.Q.GetClickTransactionByID(ctx, transactionID)
	if err != nil {
		return errorResponse(req, errTransactionNotFound)
	}
	paymentID, err := uuid.Parse(req.MerchantTransID)
	if err != nil || paymentID != tx.PaymentID {
		return errorResponse(req, errAccountNotFound)
	}
	payment, err := h.Q.GetPaymentForPayme(ctx, paymentID)
	if err != nil {
		return errorResponse(req, errAccountNotFound)
	}
	if payment.Status == "paid" {
		return errorResponse(req, errAlreadyPaid)
	}
	if tx.State == -1 {
		return errorResponse(req, errCancelled)
	}
	if payment.Status != "pending" {
		return errorResponse(req, errAccountNotFound)
	}
	amount, perr := strconv.ParseFloat(req.Amount, 64)
	if perr != nil || math.Abs(amount-float64(tx.AmountUzs)) > 0.01 {
		return errorResponse(req, errAmount)
	}
	if clickErr, _ := strconv.Atoi(req.Error); clickErr < 0 {
		if err := h.rejectTransaction(ctx, transactionID, paymentID, req.ErrorNote); err != nil {
			return errorResponse(req, errBadRequest)
		}
		return errorResponse(req, errCancelled)
	}

	alreadyConfirmed, err := h.confirmAndGrant(ctx, transactionID, paymentID)
	if err != nil {
		return errorResponse(req, errBadRequest)
	}
	_ = alreadyConfirmed // both paths return the same success shape; kept for a future distinct log line if needed
	return clickResponse{
		ClickTransID:      req.ClickTransID,
		MerchantTransID:   req.MerchantTransID,
		MerchantConfirmID: transactionID.String(),
		Error:             errSuccess,
		ErrorNote:         errorNotes[errSuccess],
	}
}

// confirmAndGrant runs the whole confirm+grant sequence in one DB
// transaction with the transaction row locked for its duration, so a
// GrantDays failure rolls back the state flip too (a retry can still
// succeed) and a concurrent second Complete call for the same
// transaction blocks until the first commits, then observes state==1
// and returns alreadyConfirmed=true instead of double-granting.
func (h *Handler) confirmAndGrant(ctx context.Context, transactionID, paymentID uuid.UUID) (alreadyConfirmed bool, err error) {
	dbtx, err := h.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = dbtx.Rollback(ctx) }()
	q := sqlc.New(dbtx)

	locked, err := q.GetClickTransactionByIDForUpdate(ctx, transactionID)
	if err != nil {
		return false, err
	}
	if locked.State == 1 {
		return true, dbtx.Commit(ctx)
	}

	if err := q.ConfirmClickTransaction(ctx, transactionID); err != nil {
		return false, err
	}
	if err := q.MarkPaymentPaid(ctx, paymentID); err != nil {
		return false, err
	}
	txSvc := billing.Service{Q: q}
	if err := txSvc.ProcessPaymentGrant(ctx, paymentID); err != nil {
		return false, err
	}
	return false, dbtx.Commit(ctx)
}

// rejectTransaction handles Click declining what it previously prepared
// (error<0 on the Complete call): it flips the click_transaction to state=-1
// and marks the payment 'canceled' together, inside their own small DB
// transaction. Lower stakes than confirmAndGrant (no money/entitlement
// involved, so no row lock is needed to prevent double-granting), but kept
// atomic and consistent with it per the plan.
func (h *Handler) rejectTransaction(ctx context.Context, transactionID, paymentID uuid.UUID, reason string) error {
	dbtx, err := h.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = dbtx.Rollback(ctx) }()
	q := sqlc.New(dbtx)
	if err := q.RejectClickTransaction(ctx, sqlc.RejectClickTransactionParams{
		ID:     transactionID,
		Reason: pgtype.Text{String: reason, Valid: reason != ""},
	}); err != nil {
		return err
	}
	if err := q.SetPaymentStatus(ctx, sqlc.SetPaymentStatusParams{ID: paymentID, Status: "canceled"}); err != nil {
		return err
	}
	return dbtx.Commit(ctx)
}
