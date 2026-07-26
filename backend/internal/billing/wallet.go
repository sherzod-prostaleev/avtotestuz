package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

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
		CardNumber:  digits,
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

// ClawbackReferralCommissionOnRefund reverses a commission for a refunded
// payment when the referrer still has enough available balance. If balance
// is insufficient (already withdrawn), writes a clawback of the available
// remainder is skipped — instead we still record a full clawback that may
// drive balance negative only when hold hasn't consumed it; if balance <
// commission we write clawback for min(balance, commission) only when
// balance > 0, else skip to avoid silent accounting lies — plan: clawback
// when not yet withdrawn. Here: if balance >= commission, full clawback;
// else if balance > 0, clawback balance; else no-op (admin must reconcile).
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
	bal, err := s.Q.GetReferralBalance(ctx, comm.ProfileID)
	if err != nil {
		return err
	}
	claw := comm.AmountUzs
	if bal < claw {
		claw = bal
	}
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
