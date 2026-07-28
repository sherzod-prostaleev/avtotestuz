package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PaymentDirectoryRow is a list row for GET /admin/v1/payments/transactions.
type PaymentDirectoryRow struct {
	ID             uuid.UUID  `json:"id"`
	ProfileID      uuid.UUID  `json:"profile_id"`
	PhoneMasked    string     `json:"phone_masked"`
	Provider       string     `json:"provider"`
	Status         string     `json:"status"`
	AmountUzs      int64      `json:"amount_uzs"`
	TariffCode     string     `json:"tariff_code"`
	IdempotencyKey string     `json:"idempotency_key"`
	CreatedAt      time.Time  `json:"created_at"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
}

// ListPaymentsResult is a paginated transactions page.
type ListPaymentsResult struct {
	Items []PaymentDirectoryRow `json:"items"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
	Total int                   `json:"total"`
}

// PaymentListFilter filters the admin payment directory.
type PaymentListFilter struct {
	Status   string
	Provider string
	From     *time.Time
	To       *time.Time
	Page     int
	Limit    int
}

// PaymentEntitlementLink summarizes VIP grant tied to a payment.
type PaymentEntitlementLink struct {
	ID        uuid.UUID `json:"id"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Active    bool      `json:"active"`
	Note      string    `json:"note,omitempty"`
}

// PaymeTxnSummary is a redacted Payme merchant txn row.
type PaymeTxnSummary struct {
	PaymeID     string `json:"payme_id"`
	State       int32  `json:"state"`
	Reason      *int32 `json:"reason,omitempty"`
	CreateTime  int64  `json:"create_time"`
	PerformTime int64  `json:"perform_time"`
	CancelTime  int64  `json:"cancel_time"`
}

// ClickTxnSummary is a redacted Click merchant txn row.
type ClickTxnSummary struct {
	ClickTransID  string     `json:"click_trans_id"`
	ClickPaydocID string     `json:"click_paydoc_id,omitempty"`
	State         int32      `json:"state"`
	Reason        string     `json:"reason,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	ConfirmedAt   *time.Time `json:"confirmed_at,omitempty"`
	RejectedAt    *time.Time `json:"rejected_at,omitempty"`
}

// RefundCapability documents whether admin can initiate a provider refund.
type RefundCapability struct {
	ActionAvailable bool   `json:"action_available"`
	ProviderPath    string `json:"provider_path"`
	Note            string `json:"note"`
}

// PaymentDetail is GET /admin/v1/payments/transactions/{id}.
type PaymentDetail struct {
	ID                   uuid.UUID                `json:"id"`
	ProfileID            uuid.UUID                `json:"profile_id"`
	PhoneMasked          string                   `json:"phone_masked"`
	TariffID             uuid.UUID                `json:"tariff_id"`
	TariffCode           string                   `json:"tariff_code"`
	TariffDaysSnapshot   int32                    `json:"tariff_days_snapshot"`
	TariffPriceSnapshot  int64                    `json:"tariff_price_uzs_snapshot"`
	AmountUzs            int64                    `json:"amount_uzs"`
	Provider             string                   `json:"provider"`
	Status               string                   `json:"status"`
	ProviderTxnID        string                   `json:"provider_txn_id,omitempty"`
	IdempotencyKey       string                   `json:"idempotency_key"`
	PromoCodeID          *uuid.UUID               `json:"promo_code_id,omitempty"`
	Meta                 map[string]any           `json:"meta"`
	CreatedAt            time.Time                `json:"created_at"`
	PaidAt               *time.Time               `json:"paid_at,omitempty"`
	Entitlement          *PaymentEntitlementLink  `json:"entitlement,omitempty"`
	Payme                *PaymeTxnSummary         `json:"payme,omitempty"`
	Click                *ClickTxnSummary         `json:"click,omitempty"`
	Refund               RefundCapability         `json:"refund"`
}

// ListPayments returns payments newest-first with status/provider/date filters.
// Extends the ops payments list shape with pagination and provider/date filters.
func (s Store) ListPayments(ctx context.Context, f PaymentListFilter) (ListPaymentsResult, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 50
	}
	status := strings.TrimSpace(strings.ToLower(f.Status))
	provider := strings.TrimSpace(strings.ToLower(f.Provider))
	offset := (f.Page - 1) * f.Limit

	var from any
	var to any
	if f.From != nil {
		from = f.From.UTC()
	}
	if f.To != nil {
		fromTo := f.To.UTC()
		to = fromTo
	}

	var total int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM payment p
		WHERE ($1 = '' OR p.status = $1)
		  AND ($2 = '' OR p.provider = $2)
		  AND ($3::timestamptz IS NULL OR p.created_at >= $3)
		  AND ($4::timestamptz IS NULL OR p.created_at < $4)`,
		status, provider, from, to).Scan(&total)
	if err != nil {
		return ListPaymentsResult{}, err
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT p.id, p.profile_id, pr.phone, p.provider, p.status, p.amount_uzs,
		       COALESCE(t.code, ''), p.idempotency_key, p.created_at, p.paid_at
		FROM payment p
		JOIN profile pr ON pr.id = p.profile_id
		LEFT JOIN tariff t ON t.id = p.tariff_id
		WHERE ($1 = '' OR p.status = $1)
		  AND ($2 = '' OR p.provider = $2)
		  AND ($3::timestamptz IS NULL OR p.created_at >= $3)
		  AND ($4::timestamptz IS NULL OR p.created_at < $4)
		ORDER BY p.created_at DESC
		LIMIT $5 OFFSET $6`,
		status, provider, from, to, f.Limit, offset)
	if err != nil {
		return ListPaymentsResult{}, err
	}
	defer rows.Close()

	items := make([]PaymentDirectoryRow, 0)
	for rows.Next() {
		var (
			row   PaymentDirectoryRow
			phone string
			paid  *time.Time
		)
		if err := rows.Scan(
			&row.ID, &row.ProfileID, &phone, &row.Provider, &row.Status, &row.AmountUzs,
			&row.TariffCode, &row.IdempotencyKey, &row.CreatedAt, &paid,
		); err != nil {
			return ListPaymentsResult{}, err
		}
		row.PhoneMasked = maskPhone(phone)
		row.CreatedAt = row.CreatedAt.UTC()
		if paid != nil {
			t := paid.UTC()
			row.PaidAt = &t
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return ListPaymentsResult{}, err
	}
	return ListPaymentsResult{Items: items, Page: f.Page, Limit: f.Limit, Total: total}, nil
}

// GetPayment returns a single payment with provider txn + entitlement context.
func (s Store) GetPayment(ctx context.Context, id uuid.UUID) (PaymentDetail, error) {
	var (
		d     PaymentDetail
		phone string
		txnID *string
		promo *uuid.UUID
		meta  []byte
		paid  *time.Time
	)
	err := s.Pool.QueryRow(ctx, `
		SELECT p.id, p.profile_id, pr.phone, p.tariff_id, COALESCE(t.code, ''),
		       p.tariff_days_snapshot, p.tariff_price_uzs_snapshot, p.amount_uzs,
		       p.provider, p.status, p.provider_txn_id, p.idempotency_key, p.promo_code_id,
		       p.meta, p.created_at, p.paid_at
		FROM payment p
		JOIN profile pr ON pr.id = p.profile_id
		LEFT JOIN tariff t ON t.id = p.tariff_id
		WHERE p.id = $1`, id).Scan(
		&d.ID, &d.ProfileID, &phone, &d.TariffID, &d.TariffCode,
		&d.TariffDaysSnapshot, &d.TariffPriceSnapshot, &d.AmountUzs,
		&d.Provider, &d.Status, &txnID, &d.IdempotencyKey, &promo,
		&meta, &d.CreatedAt, &paid,
	)
	if err != nil {
		return PaymentDetail{}, err
	}
	d.PhoneMasked = maskPhone(phone)
	d.CreatedAt = d.CreatedAt.UTC()
	if txnID != nil {
		d.ProviderTxnID = *txnID
	}
	d.PromoCodeID = promo
	if paid != nil {
		t := paid.UTC()
		d.PaidAt = &t
	}
	d.Meta = redactMeta(meta)
	d.Refund = refundCapability(d.Provider, d.Status)

	ent, err := s.loadPaymentEntitlement(ctx, id)
	if err != nil {
		return PaymentDetail{}, err
	}
	d.Entitlement = ent

	switch d.Provider {
	case "payme":
		payme, err := s.loadPaymeTxn(ctx, id)
		if err != nil {
			return PaymentDetail{}, err
		}
		d.Payme = payme
	case "click":
		click, err := s.loadClickTxn(ctx, id)
		if err != nil {
			return PaymentDetail{}, err
		}
		d.Click = click
	}
	return d, nil
}

func (s Store) loadPaymentEntitlement(ctx context.Context, paymentID uuid.UUID) (*PaymentEntitlementLink, error) {
	var (
		link PaymentEntitlementLink
		note string
	)
	err := s.Pool.QueryRow(ctx, `
		SELECT id, starts_at, ends_at, COALESCE(note, '')
		FROM entitlement
		WHERE payment_id = $1
		ORDER BY created_at DESC
		LIMIT 1`, paymentID).Scan(&link.ID, &link.StartsAt, &link.EndsAt, &note)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	link.StartsAt = link.StartsAt.UTC()
	link.EndsAt = link.EndsAt.UTC()
	link.Active = link.EndsAt.After(time.Now().UTC())
	link.Note = note
	return &link, nil
}

func (s Store) loadPaymeTxn(ctx context.Context, paymentID uuid.UUID) (*PaymeTxnSummary, error) {
	var (
		row    PaymeTxnSummary
		reason *int32
	)
	err := s.Pool.QueryRow(ctx, `
		SELECT payme_id, state, reason, create_time, perform_time, cancel_time
		FROM payme_transaction
		WHERE payment_id = $1
		ORDER BY create_time DESC
		LIMIT 1`, paymentID).Scan(
		&row.PaymeID, &row.State, &reason, &row.CreateTime, &row.PerformTime, &row.CancelTime,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	row.Reason = reason
	return &row, nil
}

func (s Store) loadClickTxn(ctx context.Context, paymentID uuid.UUID) (*ClickTxnSummary, error) {
	var (
		row       ClickTxnSummary
		paydoc    *string
		reason    *string
		confirmed *time.Time
		rejected  *time.Time
	)
	err := s.Pool.QueryRow(ctx, `
		SELECT click_trans_id, click_paydoc_id, state, reason, created_at, confirmed_at, rejected_at
		FROM click_transaction
		WHERE payment_id = $1
		ORDER BY created_at DESC
		LIMIT 1`, paymentID).Scan(
		&row.ClickTransID, &paydoc, &row.State, &reason, &row.CreatedAt, &confirmed, &rejected,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	row.CreatedAt = row.CreatedAt.UTC()
	if paydoc != nil {
		row.ClickPaydocID = *paydoc
	}
	if reason != nil {
		row.Reason = *reason
	}
	if confirmed != nil {
		t := confirmed.UTC()
		row.ConfirmedAt = &t
	}
	if rejected != nil {
		t := rejected.UTC()
		row.RejectedAt = &t
	}
	return &row, nil
}

// HardDeletePayment removes a payment and all billing rows tied to it.
// Order respects FKs (no ON DELETE CASCADE on payment children).
// Entitlements granted by this payment are deleted (VIP revoked), not soft-clamped.
// Referral ledger rows for this payment are removed so balance reflects the undo.
func (s Store) HardDeletePayment(ctx context.Context, id uuid.UUID) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var found uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM payment WHERE id = $1 FOR UPDATE`, id).Scan(&found); err != nil {
		return err
	}

	stmts := []string{
		`DELETE FROM referral_ledger WHERE payment_id = $1`,
		`DELETE FROM promo_redemption WHERE payment_id = $1`,
		`DELETE FROM entitlement WHERE payment_id = $1`,
		`UPDATE manual_pay_event
		 SET matched_payment_id = NULL,
		     status = CASE WHEN status = 'matched' THEN 'unmatched' ELSE status END
		 WHERE matched_payment_id = $1`,
		`DELETE FROM manual_pay_assignment WHERE payment_id = $1`,
		`DELETE FROM payme_transaction WHERE payment_id = $1`,
		`DELETE FROM click_transaction WHERE payment_id = $1`,
		`DELETE FROM payment WHERE id = $1`,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(ctx, q, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func refundCapability(provider, status string) RefundCapability {
	switch provider {
	case "payme":
		return RefundCapability{
			ActionAvailable: false,
			ProviderPath:    "inbound_cancel_transaction",
			Note: "Admin cannot call Payme merchant refund HTTP API from this app. " +
				"Money refunds are initiated in Payme cabinet; Payme then calls our " +
				"CancelTransaction (state=-2) which marks payment refunded and runs " +
				"RevokeEntitlementForPayment (U-04). No outbound admin refund endpoint.",
		}
	case "click":
		return RefundCapability{
			ActionAvailable: false,
			ProviderPath:    "unsupported",
			Note: "Click Merchant API has no post-paid refund webhook/RPC in this codebase. " +
				"Cabinet refunds are not wired; do not invent a Click refund action.",
		}
	default:
		return RefundCapability{
			ActionAvailable: false,
			ProviderPath:    "unsupported",
			Note:            fmt.Sprintf("No admin refund path for provider %q (status=%s).", provider, status),
		}
	}
}

func redactMeta(raw []byte) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return map[string]any{"_raw": "[unparseable]"}
	}
	for k, v := range parsed {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "secret") ||
			strings.Contains(lk, "password") ||
			strings.Contains(lk, "token") ||
			strings.Contains(lk, "key") ||
			strings.Contains(lk, "authorization") {
			out[k] = "[redacted]"
			continue
		}
		out[k] = v
	}
	return out
}
