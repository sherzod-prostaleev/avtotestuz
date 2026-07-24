package payme

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/db/sqlc"
)

// paymeTxTimeoutMs is the 12-hour window Payme allows a pending (state=1)
// transaction to stay unperformed before it must be treated as expired.
const paymeTxTimeoutMs = 12 * 60 * 60 * 1000

// cancelReasonTimeout is the Payme-defined cancel reason code for a
// transaction auto-cancelled by PerformTransaction's 12h timeout.
const cancelReasonTimeout = 4

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
		if isUniqueViolation(err) {
			// The payme_transaction_one_active_per_payment partial unique
			// index caught a race the application-level GetActivePaymeTxByPayment
			// check above missed: another transaction became active for
			// this payment between that check and this insert. This is
			// exactly the -31008 conflict, not an internal error.
			return createTransactionResult{}, errTransactionState
		}
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

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505) — e.g. the payme_transaction_one_active_per_payment
// partial unique index rejecting a second concurrent active transaction for
// the same payment. Duplicated from auth.isUniqueViolation rather than
// imported: payme has no other reason to depend on the auth package.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
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

// performTransactionParams is PerformTransaction's `params`: just the
// Payme transaction id.
type performTransactionParams struct {
	ID string `json:"id"`
}

// performTransactionResult is PerformTransaction's result, both for the
// real perform and for the idempotent replay of an already-performed one.
type performTransactionResult struct {
	Transaction string `json:"transaction"`
	PerformTime int64  `json:"perform_time"`
	State       int32  `json:"state"`
}

// performTransaction implements PerformTransaction: the transaction must
// exist (else -31003). state==2 is an idempotent replay — return the
// existing perform_time/state without granting entitlement again. A pending
// (state==1) transaction older than the 12h window is auto-cancelled
// (reason=4) and reported as -31008 rather than performed. Otherwise this is
// the real success path: flip the transaction to state=2, mark the payment
// paid, and grant the VIP entitlement via billing.Service.GrantDays — this
// is the whole point of M2-02's revenue loop. Any other state (-1/-2,
// already cancelled) can never be performed, so it's -31008 too.
//
// The whole read-decide-write sequence runs inside a single DB transaction,
// with the initial read taken via GetPaymeTransactionForUpdate (SELECT ...
// FOR UPDATE): this locks the payme_transaction row for the duration of the
// decision, so a concurrent retry of the same payme_id (Payme is documented
// to retry webhooks) blocks until this call commits, then correctly takes
// the idempotent state==2 branch instead of double-granting. Wrapping
// PerformPaymeTransaction + MarkPaymentPaid + GrantDays together also means
// a GrantDays failure rolls back the state flip: state stays 1, so a
// genuine retry can actually retry instead of silently replaying "success"
// against a customer who was never granted entitlement.
func (h *Handler) performTransaction(ctx context.Context, p performTransactionParams) (performTransactionResult, *rpcError) {
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return performTransactionResult{}, errInternal
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	existing, err := q.GetPaymeTransactionForUpdate(ctx, p.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return performTransactionResult{}, errTransactionNotFound
		}
		return performTransactionResult{}, errInternal
	}

	if existing.State == 2 {
		if err := tx.Commit(ctx); err != nil {
			return performTransactionResult{}, errInternal
		}
		return performTransactionResult{
			Transaction: existing.PaymentID.String(),
			PerformTime: existing.PerformTime,
			State:       2,
		}, nil
	}

	if existing.State != 1 {
		// -1 or -2: already cancelled, can never be performed.
		return performTransactionResult{}, errTransactionState
	}

	now := time.Now().UnixMilli()
	if existing.CreateTime+paymeTxTimeoutMs < now {
		if err := q.CancelPaymeTransaction(ctx, sqlc.CancelPaymeTransactionParams{
			PaymeID:    p.ID,
			State:      -1,
			Reason:     pgtype.Int4{Int32: cancelReasonTimeout, Valid: true},
			CancelTime: now,
		}); err != nil {
			return performTransactionResult{}, errInternal
		}
		if err := q.SetPaymentStatus(ctx, sqlc.SetPaymentStatusParams{
			ID:     existing.PaymentID,
			Status: "canceled",
		}); err != nil {
			return performTransactionResult{}, errInternal
		}
		if err := tx.Commit(ctx); err != nil {
			return performTransactionResult{}, errInternal
		}
		return performTransactionResult{}, errTransactionState
	}

	if err := q.PerformPaymeTransaction(ctx, sqlc.PerformPaymeTransactionParams{
		PaymeID:     p.ID,
		PerformTime: now,
	}); err != nil {
		return performTransactionResult{}, errInternal
	}
	if err := q.MarkPaymentPaid(ctx, existing.PaymentID); err != nil {
		return performTransactionResult{}, errInternal
	}

	txSvc := billing.Service{Q: q}
	if err := txSvc.ProcessPaymentGrant(ctx, existing.PaymentID); err != nil {
		return performTransactionResult{}, errInternal
	}

	if err := tx.Commit(ctx); err != nil {
		return performTransactionResult{}, errInternal
	}

	return performTransactionResult{
		Transaction: existing.PaymentID.String(),
		PerformTime: now,
		State:       2,
	}, nil
}

// cancelTransactionParams is CancelTransaction's `params`: Payme's
// transaction id plus an integer cancel-reason code, passed through
// verbatim to payme_transaction.reason.
type cancelTransactionParams struct {
	ID     string `json:"id"`
	Reason int32  `json:"reason"`
}

// cancelTransactionResult is CancelTransaction's result, both for a real
// cancel and for the idempotent replay of an already-cancelled one.
type cancelTransactionResult struct {
	Transaction string `json:"transaction"`
	CancelTime  int64  `json:"cancel_time"`
	State       int32  `json:"state"`
}

// cancelTransaction implements CancelTransaction: the transaction must
// exist (else -31003). state==-1/-2 is an idempotent replay — return the
// existing cancel_time/state without re-writing. A pending (state==1)
// transaction cancels to -1 and marks the payment 'canceled'; an already
// performed (state==2) transaction cancels to -2 and marks the payment
// 'refunded' — entitlement revoke on that path is deliberately deferred to
// a future refund milestone (M2-04), not implemented here.
func (h *Handler) cancelTransaction(ctx context.Context, p cancelTransactionParams) (cancelTransactionResult, *rpcError) {
	existing, err := h.Q.GetPaymeTransaction(ctx, p.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return cancelTransactionResult{}, errTransactionNotFound
		}
		return cancelTransactionResult{}, errInternal
	}

	if existing.State == -1 || existing.State == -2 {
		return cancelTransactionResult{
			Transaction: existing.PaymentID.String(),
			CancelTime:  existing.CancelTime,
			State:       existing.State,
		}, nil
	}

	var newState int32
	var paymentStatus string
	switch existing.State {
	case 1:
		newState = -1
		paymentStatus = "canceled"
	case 2:
		newState = -2
		paymentStatus = "refunded"
	default:
		// Not reachable given the 1/2/-1/-2 state machine, but guard
		// rather than silently mis-cancel an unexpected state.
		return cancelTransactionResult{}, errTransactionState
	}

	now := time.Now().UnixMilli()
	if err := h.Q.CancelPaymeTransaction(ctx, sqlc.CancelPaymeTransactionParams{
		PaymeID:    p.ID,
		State:      newState,
		Reason:     pgtype.Int4{Int32: p.Reason, Valid: true},
		CancelTime: now,
	}); err != nil {
		return cancelTransactionResult{}, errInternal
	}
	if err := h.Q.SetPaymentStatus(ctx, sqlc.SetPaymentStatusParams{
		ID:     existing.PaymentID,
		Status: paymentStatus,
	}); err != nil {
		return cancelTransactionResult{}, errInternal
	}

	return cancelTransactionResult{
		Transaction: existing.PaymentID.String(),
		CancelTime:  now,
		State:       newState,
	}, nil
}

// checkTransactionParams is CheckTransaction's `params`: just the Payme
// transaction id.
type checkTransactionParams struct {
	ID string `json:"id"`
}

// checkTransactionResult is CheckTransaction's result: the full lifecycle
// timestamps plus the current terminal state for a single transaction.
// perform_time/cancel_time are naturally 0 when unset (payme_transaction's
// columns default to 0, not NULL) and reason is 0 when unset (Reason is a
// nullable pgtype.Int4; !Valid maps to 0).
type checkTransactionResult struct {
	CreateTime  int64  `json:"create_time"`
	PerformTime int64  `json:"perform_time"`
	CancelTime  int64  `json:"cancel_time"`
	Transaction string `json:"transaction"`
	State       int32  `json:"state"`
	Reason      int32  `json:"reason"`
}

// checkTransaction implements CheckTransaction: a read-only status lookup
// by params.id, not found -> -31003. Unlike performTransaction/
// cancelTransaction this never mutates state, so a plain (non-locking,
// non-transactional) read via GetPaymeTransaction is correct — there is no
// decision to protect against a concurrent writer.
func (h *Handler) checkTransaction(ctx context.Context, p checkTransactionParams) (checkTransactionResult, *rpcError) {
	existing, err := h.Q.GetPaymeTransaction(ctx, p.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return checkTransactionResult{}, errTransactionNotFound
		}
		return checkTransactionResult{}, errInternal
	}

	var reason int32
	if existing.Reason.Valid {
		reason = existing.Reason.Int32
	}

	return checkTransactionResult{
		CreateTime:  existing.CreateTime,
		PerformTime: existing.PerformTime,
		CancelTime:  existing.CancelTime,
		Transaction: existing.PaymentID.String(),
		State:       existing.State,
		Reason:      reason,
	}, nil
}

// getStatementParams is GetStatement's `params`: an inclusive [from, to]
// window (ms) over payme_transaction.create_time.
type getStatementParams struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// statementEntry is one entry of GetStatement's result array. Time mirrors
// CreateTime: our schema never persisted Payme's own per-request `time`
// field from CreateTransaction's params (createTransaction discards
// params.Time — see its params struct's doc comment), so create_time is
// the only "when was this created" timestamp we have, and it's the
// conventional stand-in for the statement's `time` field too.
type statementEntry struct {
	ID          string        `json:"id"`
	Time        int64         `json:"time"`
	Amount      int64         `json:"amount"`
	Account     accountParams `json:"account"`
	CreateTime  int64         `json:"create_time"`
	PerformTime int64         `json:"perform_time"`
	CancelTime  int64         `json:"cancel_time"`
	Transaction string        `json:"transaction"`
	State       int32         `json:"state"`
	Reason      int32         `json:"reason"`
}

// getStatement implements GetStatement: every transaction with create_time
// in [from, to] (both inclusive), ascending by create_time. Always returns
// a non-nil slice, even for a range matching nothing, so the JSON result is
// `[]` rather than `null`.
func (h *Handler) getStatement(ctx context.Context, p getStatementParams) ([]statementEntry, *rpcError) {
	rows, err := h.Q.ListPaymeTransactionsByTime(ctx, sqlc.ListPaymeTransactionsByTimeParams{
		CreateTime:   p.From,
		CreateTime_2: p.To,
	})
	if err != nil {
		return nil, errInternal
	}

	entries := make([]statementEntry, 0, len(rows))
	for _, row := range rows {
		var reason int32
		if row.Reason.Valid {
			reason = row.Reason.Int32
		}
		entries = append(entries, statementEntry{
			ID:          row.PaymeID,
			Time:        row.CreateTime,
			Amount:      row.AmountTiyin,
			Account:     accountParams{OrderID: row.PaymentID.String()},
			CreateTime:  row.CreateTime,
			PerformTime: row.PerformTime,
			CancelTime:  row.CancelTime,
			Transaction: row.PaymentID.String(),
			State:       row.State,
			Reason:      reason,
		})
	}
	return entries, nil
}
