package click

import (
	"context"
	"errors"
	"math"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
)

// prepare implements Click's Prepare (action=0): the merchant_trans_id must
// resolve to a payment that isn't already paid, and amount (so'm, unlike
// Payme's tiyin) must match payment.amount_uzs. A click_trans_id already
// seen is an idempotent replay — return the existing merchant_prepare_id
// without touching state. Click itself signals a cancellation by sending a
// negative error code, which must be honored without creating a
// click_transaction row. Otherwise this inserts a new click_transaction
// (state=0, waiting) and moves the payment to 'pending'.
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
