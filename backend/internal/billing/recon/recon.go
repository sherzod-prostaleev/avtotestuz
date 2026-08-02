// Package recon implements a local payment↔provider-transaction consistency
// check (U-27 skeleton). It does not call live Payme/Click merchant APIs —
// those need production keys. Instead it mirrors the same windowed data
// GetStatement would expose and flags mismatches for ops / future admin queue.
package recon

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Severity classifies a finding for ops triage.
type Severity string

const (
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

// Finding is one reconciliation mismatch.
type Finding struct {
	Severity  Severity  `json:"severity"`
	Code      string    `json:"code"`
	PaymentID uuid.UUID `json:"payment_id,omitempty"`
	Provider  string    `json:"provider,omitempty"`
	Detail    string    `json:"detail"`
}

// Result summarizes a dry-run recon pass.
type Result struct {
	From       time.Time `json:"from"`
	To         time.Time `json:"to"`
	ScannedPay int       `json:"scanned_payments"`
	ScannedTxn int       `json:"scanned_provider_txns"`
	Findings   []Finding `json:"findings"`
}

// Options controls the window and pending age threshold.
type Options struct {
	From              time.Time
	To                time.Time
	StalePendingAfter time.Duration // default 12h
}

type payRow struct {
	ID        uuid.UUID
	Provider  string
	Status    string
	AmountUzs int64
	CreatedAt time.Time
}

type paymeRow struct {
	PaymeID     string
	PaymentID   uuid.UUID
	AmountTiyin int64
	State       int32
}

type clickRow struct {
	ClickTransID string
	PaymentID    uuid.UUID
	AmountUzs    int64
	State        int32
}

// Run compares local payments to provider transaction rows in [From, To].
func Run(ctx context.Context, pool *pgxpool.Pool, opts Options) (Result, error) {
	if pool == nil {
		return Result{}, fmt.Errorf("recon requires a pool")
	}
	if opts.To.IsZero() {
		opts.To = time.Now().UTC()
	}
	if opts.From.IsZero() {
		opts.From = opts.To.Add(-24 * time.Hour)
	}
	if opts.StalePendingAfter <= 0 {
		opts.StalePendingAfter = 12 * time.Hour
	}
	res := Result{From: opts.From, To: opts.To, Findings: []Finding{}}

	payments, err := listPayments(ctx, pool, opts.From, opts.To)
	if err != nil {
		return res, err
	}
	res.ScannedPay = len(payments)

	paymeByPayment, paymeCount, err := listPayme(ctx, pool, opts.From.UnixMilli(), opts.To.UnixMilli())
	if err != nil {
		return res, err
	}
	res.ScannedTxn += paymeCount

	clickByPayment, clickCount, err := listClick(ctx, pool, opts.From, opts.To)
	if err != nil {
		return res, err
	}
	res.ScannedTxn += clickCount

	now := time.Now().UTC()
	for _, p := range payments {
		switch p.Provider {
		case "payme":
			txns := paymeByPayment[p.ID]
			if len(txns) == 0 {
				// Payment may sit in-window while provider create_time drifted;
				// look up by payment_id so we don't false-positive.
				extra, err := listPaymeByPayment(ctx, pool, p.ID)
				if err != nil {
					return res, err
				}
				txns = extra
			}
			res.Findings = append(res.Findings, checkPaymePayment(p, txns, now, opts.StalePendingAfter)...)
		case "click":
			txns := clickByPayment[p.ID]
			if len(txns) == 0 {
				extra, err := listClickByPayment(ctx, pool, p.ID)
				if err != nil {
					return res, err
				}
				txns = extra
			}
			res.Findings = append(res.Findings, checkClickPayment(p, txns, now, opts.StalePendingAfter)...)
		}
	}

	// statusByPayment starts from the window's payments (already loaded
	// above by listPayments) so most payme txns resolve their payment's
	// status for free. Only ids that fall outside that set (payment
	// created_at outside the window, or genuinely missing) need a lookup,
	// and that lookup is now one batched query instead of one per txn.
	statusByPayment := make(map[uuid.UUID]string, len(payments))
	for _, p := range payments {
		statusByPayment[p.ID] = p.Status
	}
	missing := missingPaymePaymentIDs(paymeByPayment, statusByPayment)
	var lookupErr error
	if len(missing) > 0 {
		lookupErr = fillPaymentStatuses(ctx, pool, missing, statusByPayment)
	}

	for paymentID, txns := range paymeByPayment {
		for _, t := range txns {
			if t.State != 2 {
				continue
			}
			status, ok := statusByPayment[paymentID]
			if !ok {
				// Same failure surface as the old per-row
				// QueryRow(...).Scan(&status): either the batched lookup
				// itself errored (lookupErr), or it ran fine but simply
				// found no such payment (pgx.ErrNoRows, matching what
				// QueryRow returned for a missing id).
				err := lookupErr
				if err == nil {
					err = pgx.ErrNoRows
				}
				res.Findings = append(res.Findings, Finding{
					Severity: SeverityError, Code: "orphaned_payme_txn",
					PaymentID: paymentID, Provider: "payme",
					Detail: fmt.Sprintf("payme_id=%s state=2 but payment lookup failed: %v", t.PaymeID, err),
				})
				continue
			}
			if status != "paid" && status != "refunded" {
				res.Findings = append(res.Findings, Finding{
					Severity: SeverityError, Code: "perform_payment_not_paid",
					PaymentID: paymentID, Provider: "payme",
					Detail: fmt.Sprintf("payme state=2 but payment.status=%s", status),
				})
			}
		}
	}

	return res, nil
}

// missingPaymePaymentIDs returns the distinct payment ids referenced by a
// state=2 payme transaction that aren't already covered by known (i.e. not
// already loaded by listPayments), so the caller can look them up in one
// batched query instead of one per transaction.
func missingPaymePaymentIDs(paymeByPayment map[uuid.UUID][]paymeRow, known map[uuid.UUID]string) []uuid.UUID {
	var out []uuid.UUID
	for paymentID, txns := range paymeByPayment {
		if _, ok := known[paymentID]; ok {
			continue
		}
		for _, t := range txns {
			if t.State == 2 {
				out = append(out, paymentID)
				break
			}
		}
	}
	return out
}

// fillPaymentStatuses looks up payment ids not already known (e.g. the
// payment's created_at fell outside the recon window) in a single batched
// query and adds any found rows to out. The returned error (if any) is
// meant to be folded into a per-row finding by the caller, exactly like the
// old per-row QueryRow failure was — not propagated as a hard Run error.
func fillPaymentStatuses(ctx context.Context, pool *pgxpool.Pool, ids []uuid.UUID, out map[uuid.UUID]string) error {
	rows, err := pool.Query(ctx, `SELECT id, status FROM payment WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var status string
		if err := rows.Scan(&id, &status); err != nil {
			return err
		}
		out[id] = status
	}
	return rows.Err()
}

func listPayments(ctx context.Context, pool *pgxpool.Pool, from, to time.Time) ([]payRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, provider, status, amount_uzs, created_at
		FROM payment
		WHERE created_at >= $1 AND created_at <= $2
		ORDER BY created_at`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []payRow
	for rows.Next() {
		var p payRow
		if err := rows.Scan(&p.ID, &p.Provider, &p.Status, &p.AmountUzs, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func listPayme(ctx context.Context, pool *pgxpool.Pool, fromMs, toMs int64) (map[uuid.UUID][]paymeRow, int, error) {
	rows, err := pool.Query(ctx, `
		SELECT payme_id, payment_id, amount_tiyin, state
		FROM payme_transaction
		WHERE create_time >= $1 AND create_time <= $2
		ORDER BY create_time`, fromMs, toMs)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := map[uuid.UUID][]paymeRow{}
	n := 0
	for rows.Next() {
		var r paymeRow
		if err := rows.Scan(&r.PaymeID, &r.PaymentID, &r.AmountTiyin, &r.State); err != nil {
			return nil, 0, err
		}
		out[r.PaymentID] = append(out[r.PaymentID], r)
		n++
	}
	return out, n, rows.Err()
}

func listClick(ctx context.Context, pool *pgxpool.Pool, from, to time.Time) (map[uuid.UUID][]clickRow, int, error) {
	rows, err := pool.Query(ctx, `
		SELECT ct.click_trans_id, ct.payment_id, ct.amount_uzs, ct.state
		FROM click_transaction ct
		JOIN payment p ON p.id = ct.payment_id
		WHERE p.created_at >= $1 AND p.created_at <= $2`, from, to)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := map[uuid.UUID][]clickRow{}
	n := 0
	for rows.Next() {
		var r clickRow
		if err := rows.Scan(&r.ClickTransID, &r.PaymentID, &r.AmountUzs, &r.State); err != nil {
			return nil, 0, err
		}
		out[r.PaymentID] = append(out[r.PaymentID], r)
		n++
	}
	return out, n, rows.Err()
}

func checkPaymePayment(p payRow, txns []paymeRow, now time.Time, staleAfter time.Duration) []Finding {
	var out []Finding
	switch p.Status {
	case "paid":
		if !hasPaymeState(txns, 2) {
			out = append(out, Finding{
				Severity: SeverityError, Code: "paid_missing_perform",
				PaymentID: p.ID, Provider: "payme",
				Detail: "payment paid but no payme_transaction state=2",
			})
		}
		for _, t := range txns {
			if t.State == 2 && t.AmountTiyin != p.AmountUzs*100 {
				out = append(out, Finding{
					Severity: SeverityError, Code: "amount_mismatch",
					PaymentID: p.ID, Provider: "payme",
					Detail: fmt.Sprintf("payment %d so'm vs payme %d tiyin", p.AmountUzs, t.AmountTiyin),
				})
			}
		}
	case "refunded":
		if !hasPaymeState(txns, -2) && hasPaymeState(txns, 2) {
			out = append(out, Finding{
				Severity: SeverityWarn, Code: "refunded_without_cancel",
				PaymentID: p.ID, Provider: "payme",
				Detail: "payment refunded but payme txn not state=-2",
			})
		}
	case "pending", "created":
		if now.Sub(p.CreatedAt) > staleAfter {
			out = append(out, Finding{
				Severity: SeverityWarn, Code: "stale_pending",
				PaymentID: p.ID, Provider: "payme",
				Detail: fmt.Sprintf("pending/created older than %s", staleAfter),
			})
		}
	}
	return out
}

func checkClickPayment(p payRow, txns []clickRow, now time.Time, staleAfter time.Duration) []Finding {
	var out []Finding
	switch p.Status {
	case "paid":
		if !hasClickState(txns, 1) {
			out = append(out, Finding{
				Severity: SeverityError, Code: "paid_missing_confirm",
				PaymentID: p.ID, Provider: "click",
				Detail: "payment paid but no click_transaction state=1",
			})
		}
		for _, t := range txns {
			if t.State == 1 && t.AmountUzs != p.AmountUzs {
				out = append(out, Finding{
					Severity: SeverityError, Code: "amount_mismatch",
					PaymentID: p.ID, Provider: "click",
					Detail: fmt.Sprintf("payment %d vs click %d so'm", p.AmountUzs, t.AmountUzs),
				})
			}
		}
	case "pending", "created":
		if now.Sub(p.CreatedAt) > staleAfter {
			out = append(out, Finding{
				Severity: SeverityWarn, Code: "stale_pending",
				PaymentID: p.ID, Provider: "click",
				Detail: fmt.Sprintf("pending/created older than %s", staleAfter),
			})
		}
	}
	return out
}

func listPaymeByPayment(ctx context.Context, pool *pgxpool.Pool, paymentID uuid.UUID) ([]paymeRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT payme_id, payment_id, amount_tiyin, state
		FROM payme_transaction WHERE payment_id = $1`, paymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []paymeRow
	for rows.Next() {
		var r paymeRow
		if err := rows.Scan(&r.PaymeID, &r.PaymentID, &r.AmountTiyin, &r.State); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func listClickByPayment(ctx context.Context, pool *pgxpool.Pool, paymentID uuid.UUID) ([]clickRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT click_trans_id, payment_id, amount_uzs, state
		FROM click_transaction WHERE payment_id = $1`, paymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []clickRow
	for rows.Next() {
		var r clickRow
		if err := rows.Scan(&r.ClickTransID, &r.PaymentID, &r.AmountUzs, &r.State); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func hasPaymeState(txns []paymeRow, state int32) bool {
	for _, t := range txns {
		if t.State == state {
			return true
		}
	}
	return false
}

func hasClickState(txns []clickRow, state int32) bool {
	for _, t := range txns {
		if t.State == state {
			return true
		}
	}
	return false
}
