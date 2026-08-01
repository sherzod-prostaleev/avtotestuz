package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"avtotest.uz/backend/internal/db/sqlc"
)

var (
	ErrPayoutInsufficientBalance = errors.New("insufficient referral balance")
	ErrPayoutInvalidAmount       = errors.New("payout amount must be positive")
	ErrPayoutInvalidCard         = errors.New("invalid card number")
	ErrPayoutInvalidNetwork      = errors.New("card_network must be uzcard or humo")
	ErrPayoutNotFound            = errors.New("payout not found")
	ErrPayoutNotPending          = errors.New("payout is not pending")
)

var digitsOnly = regexp.MustCompile(`^\d{16}$`)

// BuyerReferralBonusDays returns extra VIP days for a referee who buys via
// an attributed referral. Mapped to seeded tariffs: nexia/gentra/malibu.
func BuyerReferralBonusDays(tariffDays int32) int {
	switch tariffDays {
	case 7:
		return 3
	case 30:
		return 7
	case 75:
		return 10
	default:
		return 0
	}
}

func CommissionUZS(amountUzs int64, percent int32) int64 {
	if amountUzs <= 0 || percent <= 0 {
		return 0
	}
	return amountUzs * int64(percent) / 100
}

func NormalizeCardNumber(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func MaskCardNumber(digits string) string {
	if len(digits) < 4 {
		return "****"
	}
	return "**** **** **** " + digits[len(digits)-4:]
}

func DetectCardNetwork(digits string) string {
	if len(digits) < 4 {
		return ""
	}
	prefix2 := digits[:2]
	// Humo BINs commonly start with 9860; Uzcard with 8600.
	if strings.HasPrefix(digits, "9860") || prefix2 == "98" {
		return "humo"
	}
	if strings.HasPrefix(digits, "8600") || prefix2 == "86" {
		return "uzcard"
	}
	return ""
}

func ValidatePayoutCard(cardNumber, network string) (digits string, net string, err error) {
	digits = NormalizeCardNumber(cardNumber)
	if !digitsOnly.MatchString(digits) {
		return "", "", ErrPayoutInvalidCard
	}
	net = strings.ToLower(strings.TrimSpace(network))
	if net != "uzcard" && net != "humo" {
		return "", "", ErrPayoutInvalidNetwork
	}
	detected := DetectCardNetwork(digits)
	if detected != "" && detected != net {
		return "", "", ErrPayoutInvalidCard
	}
	return digits, net, nil
}

func (s Service) ReferralBalance(ctx context.Context, profileID uuid.UUID) (int64, error) {
	return s.Q.GetReferralBalance(ctx, profileID)
}

func maskPhoneLast4(phone string) string {
	digits := NormalizeCardNumber(phone) // reuse digit strip
	if len(digits) < 4 {
		if phone == "" {
			return "—"
		}
		return "***"
	}
	return "***" + digits[len(digits)-4:]
}

type PayoutSummary struct {
	PendingCount  int64 `json:"pending_count"`
	PendingUzs    int64 `json:"pending_uzs"`
	PaidCount     int64 `json:"paid_count"`
	PaidUzs       int64 `json:"paid_uzs"`
	RejectedCount int64 `json:"rejected_count"`
	RejectedUzs   int64 `json:"rejected_uzs"`
}

type UserPayoutRow struct {
	ID          uuid.UUID `json:"id"`
	AmountUzs   int64     `json:"amount_uzs"`
	CardMasked  string    `json:"card_masked"`
	CardNetwork string    `json:"card_network"`
	Status      string    `json:"status"`
	AdminNote   string    `json:"admin_note,omitempty"`
	CreatedAt   string    `json:"created_at"`
	ProcessedAt *string   `json:"processed_at,omitempty"`
}

type EarningRow struct {
	LedgerID         uuid.UUID `json:"ledger_id"`
	CommissionUzs    int64     `json:"commission_uzs"`
	PaymentAmountUzs int64     `json:"payment_amount_uzs"`
	TariffCode       string    `json:"tariff_code"`
	TariffDays       int32     `json:"tariff_days"`
	PercentSnapshot  int32     `json:"percent_snapshot"`
	RefereeLabel     string    `json:"referee_label"`
	RewardedAt       string    `json:"rewarded_at"`
}

type ReferralActivity struct {
	PayoutSummary PayoutSummary   `json:"payout_summary"`
	Payouts       []UserPayoutRow `json:"payouts"`
	Earnings      []EarningRow    `json:"earnings"`
}

func (s Service) GetReferralActivity(ctx context.Context, profileID uuid.UUID) (*ReferralActivity, error) {
	sum, err := s.Q.GetReferralPayoutSummaryForProfile(ctx, profileID)
	if err != nil {
		return nil, err
	}
	payoutRows, err := s.Q.ListReferralPayoutsForProfile(ctx, sqlc.ListReferralPayoutsForProfileParams{
		ProfileID: profileID,
		Limit:     50,
	})
	if err != nil {
		return nil, err
	}
	earnRows, err := s.Q.ListReferralEarningsDetail(ctx, sqlc.ListReferralEarningsDetailParams{
		ProfileID: profileID,
		Limit:     50,
	})
	if err != nil {
		return nil, err
	}

	out := &ReferralActivity{
		PayoutSummary: PayoutSummary{
			PendingCount:  sum.PendingCount,
			PendingUzs:    sum.PendingUzs,
			PaidCount:     sum.PaidCount,
			PaidUzs:       sum.PaidUzs,
			RejectedCount: sum.RejectedCount,
			RejectedUzs:   sum.RejectedUzs,
		},
		Payouts:  make([]UserPayoutRow, 0, len(payoutRows)),
		Earnings: make([]EarningRow, 0, len(earnRows)),
	}
	for _, row := range payoutRows {
		item := UserPayoutRow{
			ID:          row.ID,
			AmountUzs:   row.AmountUzs,
			CardMasked:  MaskStoredPAN(row.CardNumber),
			CardNetwork: row.CardNetwork,
			Status:      row.Status,
			AdminNote:   row.AdminNote,
			CreatedAt:   row.CreatedAt.Time.UTC().Format(time.RFC3339),
		}
		if row.ProcessedAt.Valid {
			t := row.ProcessedAt.Time.UTC().Format(time.RFC3339)
			item.ProcessedAt = &t
		}
		out.Payouts = append(out.Payouts, item)
	}
	for _, row := range earnRows {
		label := strings.TrimSpace(row.RefereeName)
		if label == "" {
			label = maskPhoneLast4(row.RefereePhone)
		} else {
			label = label + " · " + maskPhoneLast4(row.RefereePhone)
		}
		out.Earnings = append(out.Earnings, EarningRow{
			LedgerID:         row.LedgerID,
			CommissionUzs:    row.CommissionUzs,
			PaymentAmountUzs: row.PaymentAmountUzs,
			TariffCode:       row.TariffCode,
			TariffDays:       row.TariffDays,
			PercentSnapshot:  row.PercentSnapshot,
			RefereeLabel:     label,
			RewardedAt:       row.RewardedAt.Time.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

type LedgerEntry struct {
	ID        uuid.UUID       `json:"id"`
	EntryType string          `json:"entry_type"`
	AmountUzs int64           `json:"amount_uzs"`
	PaymentID *uuid.UUID      `json:"payment_id,omitempty"`
	PayoutID  *uuid.UUID      `json:"payout_id,omitempty"`
	Meta      json.RawMessage `json:"meta,omitempty"`
	CreatedAt string          `json:"created_at"`
}

func (s Service) ListReferralLedger(ctx context.Context, profileID uuid.UUID, limit int32) ([]LedgerEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.Q.ListReferralLedgerForUser(ctx, sqlc.ListReferralLedgerForUserParams{
		ProfileID: profileID,
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]LedgerEntry, 0, len(rows))
	for _, row := range rows {
		e := LedgerEntry{
			ID:        row.ID,
			EntryType: row.EntryType,
			AmountUzs: row.AmountUzs,
			Meta:      row.Meta,
			CreatedAt: row.CreatedAt.Time.UTC().Format("2006-01-02T15:04:05Z"),
		}
		if row.PaymentID.Valid {
			id := row.PaymentID.UUID
			e.PaymentID = &id
		}
		if row.PayoutID.Valid {
			id := row.PayoutID.UUID
			e.PayoutID = &id
		}
		out = append(out, e)
	}
	return out, nil
}

type PayoutResult struct {
	ID          uuid.UUID `json:"id"`
	AmountUzs   int64     `json:"amount_uzs"`
	CardMasked  string    `json:"card_masked"`
	CardNetwork string    `json:"card_network"`
	Status      string    `json:"status"`
}

func (s Service) RequestReferralPayout(ctx context.Context, profileID uuid.UUID, amountUzs int64, cardNumber, cardNetwork string) (*PayoutResult, error) {
	if amountUzs <= 0 {
		return nil, ErrPayoutInvalidAmount
	}
	digits, net, err := ValidatePayoutCard(cardNumber, cardNetwork)
	if err != nil {
		return nil, err
	}
	if s.Pool == nil {
		return nil, fmt.Errorf("billing: RequestReferralPayout requires Service.Pool")
	}
	cardCiphertext, _, err := s.EncryptPAN(digits)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(tx)

	if _, err := q.LockProfileForGrant(ctx, profileID); err != nil {
		return nil, fmt.Errorf("lock profile: %w", err)
	}
	bal, err := q.GetReferralBalance(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if bal < amountUzs {
		return nil, ErrPayoutInsufficientBalance
	}

	payout, err := q.CreateReferralPayout(ctx, sqlc.CreateReferralPayoutParams{
		ProfileID:   profileID,
		AmountUzs:   amountUzs,
		CardNumber:  cardCiphertext,
		CardNetwork: net,
	})
	if err != nil {
		return nil, fmt.Errorf("create payout: %w", err)
	}
	meta, _ := json.Marshal(map[string]any{
		"card_masked":  MaskCardNumber(digits),
		"card_network": net,
	})
	if _, err := q.InsertReferralLedger(ctx, sqlc.InsertReferralLedgerParams{
		ProfileID: profileID,
		EntryType: "payout_hold",
		AmountUzs: -amountUzs,
		PaymentID: uuid.NullUUID{},
		PayoutID:  uuid.NullUUID{UUID: payout.ID, Valid: true},
		Meta:      meta,
	}); err != nil {
		return nil, fmt.Errorf("hold ledger: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &PayoutResult{
		ID:          payout.ID,
		AmountUzs:   payout.AmountUzs,
		CardMasked:  MaskCardNumber(digits),
		CardNetwork: payout.CardNetwork,
		Status:      payout.Status,
	}, nil
}

func (s Service) creditReferralCommission(ctx context.Context, referrerID, refereeID, paymentID uuid.UUID, amountUzs int64, percent int32, tariffDays int32) error {
	commission := CommissionUZS(amountUzs, percent)
	if commission <= 0 {
		return nil
	}
	meta, _ := json.Marshal(map[string]any{
		"referee_id":  refereeID.String(),
		"percent":     percent,
		"amount_uzs":  amountUzs,
		"tariff_days": tariffDays,
	})
	_, err := s.Q.InsertReferralLedger(ctx, sqlc.InsertReferralLedgerParams{
		ProfileID: referrerID,
		EntryType: "commission",
		AmountUzs: commission,
		PaymentID: uuid.NullUUID{UUID: paymentID, Valid: true},
		PayoutID:  uuid.NullUUID{},
		Meta:      meta,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("insert commission: %w", err)
	}
	return nil
}

// ClawbackReferralCommissionOnRefund reverses a commission when its payment
// is refunded. It always writes the FULL commission back, even when that
// drives the balance negative.
//
// It used to clamp the clawback to the current balance and skip entirely
// when that balance was <= 0. Because a pending payout writes a negative
// payout_hold row, merely *requesting* a withdrawal drove the balance to 0
// and silently cancelled the clawback — so "refer yourself, buy, request a
// payout, then refund the purchase" extracted the commission in cash with
// nothing left to reverse it, repeatably.
//
// A negative balance is the correct accounting outcome: RequestReferralPayout
// refuses while bal < amount, so the debt simply blocks further withdrawals
// until future commissions work it off.
//
// Idempotent per payment via the clawback unique index and the early return
// below. Callers must supply a transaction-bound Service — the profile lock
// only holds for the caller's transaction.
func (s Service) ClawbackReferralCommissionOnRefund(ctx context.Context, paymentID uuid.UUID) error {
	comm, err := s.Q.GetCommissionByPaymentID(ctx, uuid.NullUUID{UUID: paymentID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if _, err := s.Q.GetClawbackByPaymentID(ctx, uuid.NullUUID{UUID: paymentID, Valid: true}); err == nil {
		return nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if _, err := s.Q.LockProfileForGrant(ctx, comm.ProfileID); err != nil {
		return err
	}
	claw := comm.AmountUzs
	if claw <= 0 {
		return nil
	}
	meta, _ := json.Marshal(map[string]any{
		"reason":            "payment_refunded",
		"original_payment":  paymentID.String(),
		"commission_amount": comm.AmountUzs,
	})
	_, err = s.Q.InsertReferralLedger(ctx, sqlc.InsertReferralLedgerParams{
		ProfileID: comm.ProfileID,
		EntryType: "clawback",
		AmountUzs: -claw,
		PaymentID: uuid.NullUUID{UUID: paymentID, Valid: true},
		PayoutID:  uuid.NullUUID{},
		Meta:      meta,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil
		}
		return err
	}
	return nil
}
