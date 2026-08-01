package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
)

const ManualHoldDuration = 10 * time.Minute

// manualAmountSlots is how many distinct som suffixes an assignment may add
// to the tariff price so that every open assignment on a card is identified
// by its exact sum. 100 slots x 4 cards is far beyond real concurrency, and
// the largest nudge (+99 som, about a US cent) is immaterial to the payer.
const manualAmountSlots = 100

var (
	ErrManualNoCardAvailable = errors.New("no manual pay card available")
	ErrManualNotFound        = errors.New("manual payment not found")
	ErrManualWrongOwner      = errors.New("manual payment belongs to another profile")
	ErrManualAlreadyDone     = errors.New("manual payment already finalized")
	ErrManualInvalidCard     = errors.New("invalid card number")
	ErrManualRejectPaid      = errors.New("cannot reject a paid manual payment")
)

// ManualCheckoutInfo is returned instead of an external checkout URL.
type ManualCheckoutInfo struct {
	PaymentID   uuid.UUID `json:"payment_id"`
	AmountUzs   int64     `json:"amount_uzs"`
	PanFull     string    `json:"pan_full"`
	PanLast4    string    `json:"pan_last4"`
	HolderName  string    `json:"holder_name"`
	Network     string    `json:"network"`
	HoldUntil   time.Time `json:"hold_until"`
	ManualState string    `json:"manual_state"`
}

type ManualPaymentStatus struct {
	PaymentID     uuid.UUID  `json:"payment_id"`
	PaymentStatus string     `json:"payment_status"`
	ManualState   string     `json:"manual_state"`
	AmountUzs     int64      `json:"amount_uzs"`
	PanFull       string     `json:"pan_full"`
	PanLast4      string     `json:"pan_last4"`
	HolderName    string     `json:"holder_name"`
	Network       string     `json:"network"`
	HoldUntil     time.Time  `json:"hold_until"`
	ReleasedAt    *time.Time `json:"released_at,omitempty"`
	ClaimedAt     *time.Time `json:"claimed_at,omitempty"`
	ConfirmedBy   string     `json:"confirmed_by,omitempty"`
}

// StartManualCheckout creates payment + assigns a free Humo card (10m hold).
func (s Service) StartManualCheckout(ctx context.Context, profileID uuid.UUID, tariffCode, promoCode string) (ManualCheckoutInfo, error) {
	if s.Pool == nil {
		return ManualCheckoutInfo{}, fmt.Errorf("billing: StartManualCheckout requires Pool")
	}
	if err := s.EnsureProviderEnabled(ctx, "manual"); err != nil {
		return ManualCheckoutInfo{}, err
	}

	tariff, err := s.Q.GetActiveTariffByCode(ctx, tariffCode)
	if err != nil {
		return ManualCheckoutInfo{}, fmt.Errorf("tariff %q: %w", tariffCode, err)
	}

	dbtx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ManualCheckoutInfo{}, err
	}
	defer func() { _ = dbtx.Rollback(ctx) }()
	q := sqlc.New(dbtx)
	txSvc := Service{Q: q, Pool: s.Pool, PublicBaseURL: s.PublicBaseURL, Secret: s.Secret}

	amount := tariff.PriceUzs
	var promoID uuid.NullUUID
	if promoCode != "" {
		wasReferral, refErr := txSvc.TryApplyReferralAtCheckout(ctx, profileID, promoCode)
		if refErr != nil {
			return ManualCheckoutInfo{}, refErr
		}
		if !wasReferral {
			valRes, err := txSvc.validatePromoLocked(ctx, profileID, promoCode, tariffCode)
			if err != nil {
				return ManualCheckoutInfo{}, fmt.Errorf("validate promo: %w", err)
			}
			amount = valRes.FinalAmountUzs
			promoID = uuid.NullUUID{UUID: valRes.PromoID, Valid: true}
		}
	}
	if amount <= 0 {
		return ManualCheckoutInfo{}, fmt.Errorf("manual checkout requires positive amount")
	}

	if err := q.ReleaseExpiredManualHolds(ctx); err != nil {
		return ManualCheckoutInfo{}, fmt.Errorf("release holds: %w", err)
	}
	card, err := q.LockFreeManualPayCard(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ManualCheckoutInfo{}, ErrManualNoCardAvailable
		}
		return ManualCheckoutInfo{}, err
	}
	panFull, err := s.DecryptPAN(card.PanFull)
	if err != nil {
		return ManualCheckoutInfo{}, fmt.Errorf("decrypt assigned card: %w", err)
	}

	// Make the expected sum unique among the assignments currently open on
	// this card. Matching keys on (card, amount), so two users owing the same
	// tariff price on the same card were previously indistinguishable and the
	// newest assignment captured the other's transfer.
	amount, err = uniqueManualAmount(ctx, q, card.ID, amount)
	if err != nil {
		return ManualCheckoutInfo{}, err
	}

	paymentID, err := q.CreatePayment(ctx, sqlc.CreatePaymentParams{
		ProfileID:              profileID,
		TariffID:               tariff.ID,
		AmountUzs:              amount,
		Provider:               "manual",
		IdempotencyKey:         uuid.NewString(),
		PromoCodeID:            promoID,
		TariffDaysSnapshot:     tariff.Days,
		TariffPriceUzsSnapshot: tariff.PriceUzs,
	})
	if err != nil {
		return ManualCheckoutInfo{}, fmt.Errorf("create payment: %w", err)
	}
	if err := q.SetPaymentPending(ctx, paymentID); err != nil {
		return ManualCheckoutInfo{}, err
	}

	now := time.Now().UTC()
	holdUntil := now.Add(ManualHoldDuration)
	asg, err := q.InsertManualPayAssignment(ctx, sqlc.InsertManualPayAssignmentParams{
		PaymentID:  paymentID,
		CardID:     card.ID,
		AmountUzs:  amount,
		AssignedAt: pgtype.Timestamptz{Time: now, Valid: true},
		HoldUntil:  pgtype.Timestamptz{Time: holdUntil, Valid: true},
	})
	if err != nil {
		return ManualCheckoutInfo{}, fmt.Errorf("assign card: %w", err)
	}
	if err := dbtx.Commit(ctx); err != nil {
		return ManualCheckoutInfo{}, err
	}
	return ManualCheckoutInfo{
		PaymentID:   paymentID,
		AmountUzs:   amount,
		PanFull:     panFull,
		PanLast4:    card.PanLast4,
		HolderName:  card.HolderName,
		Network:     card.Network,
		HoldUntil:   asg.HoldUntil.Time,
		ManualState: asg.ManualState,
	}, nil
}

func (s Service) GetManualPaymentStatus(ctx context.Context, profileID, paymentID uuid.UUID) (ManualPaymentStatus, error) {
	if s.Pool != nil {
		_ = s.Q.ReleaseExpiredManualHolds(ctx)
	}
	row, err := s.Q.GetManualPayAssignmentByPayment(ctx, paymentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ManualPaymentStatus{}, ErrManualNotFound
		}
		return ManualPaymentStatus{}, err
	}
	pay, err := s.Q.GetPaymentForPayme(ctx, paymentID)
	if err != nil {
		return ManualPaymentStatus{}, err
	}
	if pay.ProfileID != profileID {
		return ManualPaymentStatus{}, ErrManualWrongOwner
	}
	out := ManualPaymentStatus{
		PaymentID:     paymentID,
		PaymentStatus: pay.Status,
		ManualState:   row.ManualState,
		AmountUzs:     row.AmountUzs,
		PanFull:       "",
		PanLast4:      row.PanLast4,
		HolderName:    row.HolderName,
		Network:       row.Network,
		HoldUntil:     row.HoldUntil.Time,
	}
	out.PanFull, err = s.DecryptPAN(row.PanFull)
	if err != nil {
		return ManualPaymentStatus{}, fmt.Errorf("decrypt assigned card: %w", err)
	}
	if row.ReleasedAt.Valid {
		t := row.ReleasedAt.Time
		out.ReleasedAt = &t
	}
	if row.ClaimedAt.Valid {
		t := row.ClaimedAt.Time
		out.ClaimedAt = &t
	}
	if row.ConfirmedBy.Valid {
		out.ConfirmedBy = row.ConfirmedBy.String
	}
	return out, nil
}

func (s Service) ClaimManualPayment(ctx context.Context, profileID, paymentID uuid.UUID) (ManualPaymentStatus, error) {
	if s.Pool == nil {
		return ManualPaymentStatus{}, fmt.Errorf("billing: ClaimManualPayment requires Pool")
	}
	dbtx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ManualPaymentStatus{}, err
	}
	defer func() { _ = dbtx.Rollback(ctx) }()
	q := sqlc.New(dbtx)
	_ = q.ReleaseExpiredManualHolds(ctx)

	pay, err := q.GetPaymentForPayme(ctx, paymentID)
	if err != nil {
		return ManualPaymentStatus{}, ErrManualNotFound
	}
	if pay.ProfileID != profileID {
		return ManualPaymentStatus{}, ErrManualWrongOwner
	}
	if pay.Status == "paid" || pay.Status == "canceled" || pay.Status == "refunded" {
		return ManualPaymentStatus{}, ErrManualAlreadyDone
	}
	asg, err := q.GetManualPayAssignmentByPaymentForUpdate(ctx, paymentID)
	if err != nil {
		return ManualPaymentStatus{}, ErrManualNotFound
	}
	if asg.ManualState == "consumed" || asg.ManualState == "rejected" {
		return ManualPaymentStatus{}, ErrManualAlreadyDone
	}
	if err := q.ClaimManualPayAssignment(ctx, paymentID); err != nil {
		return ManualPaymentStatus{}, err
	}
	if err := dbtx.Commit(ctx); err != nil {
		return ManualPaymentStatus{}, err
	}
	return s.GetManualPaymentStatus(ctx, profileID, paymentID)
}

// ConfirmManualPayment marks paid + grants (idempotent). source is "bot" or "admin".
func (s Service) ConfirmManualPayment(ctx context.Context, paymentID uuid.UUID, source string) (already bool, err error) {
	if s.Pool == nil {
		return false, fmt.Errorf("billing: ConfirmManualPayment requires Pool")
	}
	if source != "bot" && source != "admin" {
		return false, fmt.Errorf("invalid confirm source")
	}
	dbtx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = dbtx.Rollback(ctx) }()
	q := sqlc.New(dbtx)

	asg, err := q.GetManualPayAssignmentByPaymentForUpdate(ctx, paymentID)
	if err != nil {
		return false, ErrManualNotFound
	}
	pay, err := q.GetPaymentForPayme(ctx, paymentID)
	if err != nil {
		return false, ErrManualNotFound
	}
	if pay.Status == "paid" || asg.ManualState == "consumed" {
		return true, dbtx.Commit(ctx)
	}
	if asg.ManualState == "rejected" || pay.Status == "canceled" || pay.Status == "refunded" {
		return false, ErrManualAlreadyDone
	}

	if err := q.MarkPaymentPaid(ctx, paymentID); err != nil {
		return false, err
	}
	txSvc := Service{Q: q}
	if err := txSvc.ProcessPaymentGrant(ctx, paymentID); err != nil {
		return false, err
	}
	if err := q.MarkManualPayAssignmentConsumed(ctx, sqlc.MarkManualPayAssignmentConsumedParams{
		PaymentID:   paymentID,
		ConfirmedBy: pgtype.Text{String: source, Valid: true},
	}); err != nil {
		return false, err
	}
	return false, dbtx.Commit(ctx)
}

func (s Service) RejectManualPayment(ctx context.Context, paymentID uuid.UUID, note string) error {
	if s.Pool == nil {
		return fmt.Errorf("billing: RejectManualPayment requires Pool")
	}
	dbtx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = dbtx.Rollback(ctx) }()
	q := sqlc.New(dbtx)

	asg, err := q.GetManualPayAssignmentByPaymentForUpdate(ctx, paymentID)
	if err != nil {
		return ErrManualNotFound
	}
	pay, err := q.GetPaymentForPayme(ctx, paymentID)
	if err != nil {
		return ErrManualNotFound
	}
	if pay.Status == "paid" || asg.ManualState == "consumed" {
		return ErrManualRejectPaid
	}
	if err := q.RejectManualPayAssignment(ctx, sqlc.RejectManualPayAssignmentParams{
		PaymentID: paymentID,
		AdminNote: pgtype.Text{String: note, Valid: note != ""},
	}); err != nil {
		return err
	}
	if err := q.SetPaymentCanceled(ctx, paymentID); err != nil {
		return err
	}
	return dbtx.Commit(ctx)
}

// IngestHumoPush stores and optionally auto-confirms a Humocardbot push.
func (s Service) IngestHumoPush(ctx context.Context, rawText string, telegramMsgID int64) (map[string]any, error) {
	if s.Pool == nil {
		return nil, fmt.Errorf("billing: IngestHumoPush requires Pool")
	}
	fp := humoPushFingerprint(telegramMsgID, rawText)
	if existing, err := s.Q.GetManualPayEventByFingerprint(ctx, fp); err == nil {
		return map[string]any{
			"event_id":           existing.ID,
			"status":             existing.Status,
			"matched_payment_id": nullUUID(existing.MatchedPaymentID),
			"duplicate":          true,
		}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	parsed, ok := ParseHumoCardBotPush(rawText)
	var amount pgtype.Int8
	var last4 pgtype.Text
	var transferAt pgtype.Timestamptz
	var balance pgtype.Int8
	status := "unmatched"
	if ok {
		amount = pgtype.Int8{Int64: parsed.AmountUzs, Valid: true}
		last4 = pgtype.Text{String: parsed.PanLast4, Valid: true}
		transferAt = pgtype.Timestamptz{Time: parsed.TransferAt.UTC(), Valid: true}
		if parsed.HasBalance {
			balance = pgtype.Int8{Int64: parsed.BalanceUzs, Valid: true}
		}
	}

	var msgID pgtype.Int8
	if telegramMsgID != 0 {
		msgID = pgtype.Int8{Int64: telegramMsgID, Valid: true}
	}

	ev, err := s.Q.InsertManualPayEvent(ctx, sqlc.InsertManualPayEventParams{
		Fingerprint:      fp,
		TelegramMsgID:    msgID,
		RawText:          rawText,
		AmountUzs:        amount,
		PanLast4:         last4,
		TransferAt:       transferAt,
		BalanceUzs:       balance,
		Status:           status,
		MatchedPaymentID: uuid.NullUUID{},
		ParseOk:          ok,
	})
	if err != nil {
		if isUniqueViolation(err) {
			existing, gerr := s.Q.GetManualPayEventByFingerprint(ctx, fp)
			if gerr != nil {
				return nil, gerr
			}
			return map[string]any{
				"event_id":           existing.ID,
				"status":             existing.Status,
				"matched_payment_id": nullUUID(existing.MatchedPaymentID),
				"duplicate":          true,
			}, nil
		}
		return nil, err
	}
	if !ok {
		return map[string]any{"event_id": ev.ID, "status": "unmatched", "parse_ok": false}, nil
	}

	_ = s.Q.ReleaseExpiredManualHolds(ctx)
	cand, err := s.Q.FindManualPayMatchCandidate(ctx, sqlc.FindManualPayMatchCandidateParams{
		PanLast4:   parsed.PanLast4,
		AmountUzs:  parsed.AmountUzs,
		TransferAt: pgtype.Timestamptz{Time: parsed.TransferAt.UTC(), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]any{"event_id": ev.ID, "status": "unmatched", "parse_ok": true}, nil
		}
		return nil, err
	}

	already, cerr := s.ConfirmManualPayment(ctx, cand.PaymentID, "bot")
	if cerr != nil {
		if errors.Is(cerr, ErrManualAlreadyDone) {
			return map[string]any{
				"event_id":           ev.ID,
				"status":             "unmatched",
				"parse_ok":           true,
				"match_skipped":      true,
				"matched_payment_id": cand.PaymentID,
			}, nil
		}
		return nil, cerr
	}
	_ = s.Q.MarkManualPayEventMatched(ctx, sqlc.MarkManualPayEventMatchedParams{
		ID:               ev.ID,
		MatchedPaymentID: uuid.NullUUID{UUID: cand.PaymentID, Valid: true},
	})
	return map[string]any{
		"event_id":           ev.ID,
		"status":             "matched",
		"matched_payment_id": cand.PaymentID,
		"already_confirmed":  already,
		"parse_ok":           true,
	}, nil
}

func humoPushFingerprint(msgID int64, raw string) string {
	h := sha256.New()
	// hash.Hash writes never fail, hence the discards.
	_, _ = fmt.Fprintf(h, "%d\n", msgID)
	_, _ = h.Write([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(h.Sum(nil))
}

func nullUUID(n uuid.NullUUID) any {
	if !n.Valid {
		return nil
	}
	return n.UUID
}

func NormalizeCardPAN(pan string) (full string, last4 string, err error) {
	var b strings.Builder
	for _, r := range pan {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if len(digits) < 16 || len(digits) > 19 {
		return "", "", ErrManualInvalidCard
	}
	return digits, digits[len(digits)-4:], nil
}

func FormatCardDisplay(pan string) string {
	digits := pan
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && i%4 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// uniqueManualAmount returns base plus the smallest offset in [0,
// manualAmountSlots) that no open assignment on this card is already
// expecting. Amounts only need to be unique per card, since matching is
// keyed on (pan_last4, amount).
func uniqueManualAmount(ctx context.Context, q *sqlc.Queries, cardID uuid.UUID, base int64) (int64, error) {
	taken, err := q.ListOpenManualAmountsForCard(ctx, cardID)
	if err != nil {
		return 0, fmt.Errorf("list open amounts: %w", err)
	}
	used := make(map[int64]struct{}, len(taken))
	for _, amt := range taken {
		used[amt] = struct{}{}
	}
	for offset := int64(0); offset < manualAmountSlots; offset++ {
		candidate := base + offset
		if _, clash := used[candidate]; !clash {
			return candidate, nil
		}
	}
	// Every suffix for this price is in flight on this card. Treat it like a
	// busy pool rather than handing out a colliding amount.
	return 0, ErrManualNoCardAvailable
}
