package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/db/sqlc"
)

var (
	ErrPromoNotFound         = errors.New("promo_not_found")
	ErrPromoExpired          = errors.New("promo_expired")
	ErrPromoNotStarted       = errors.New("promo_not_started")
	ErrPromoLimitReached     = errors.New("promo_limit_reached")
	ErrPromoUserLimitReached = errors.New("promo_user_limit_reached")
)

type ValidatePromoResult struct {
	PromoID           uuid.UUID `json:"promo_id"`
	Code              string    `json:"code"`
	Kind              string    `json:"kind"`
	Value             int       `json:"value"`
	DiscountUzs       int64     `json:"discount_uzs"`
	OriginalAmountUzs int64     `json:"original_amount_uzs"`
	FinalAmountUzs    int64     `json:"final_amount_uzs"`
	BonusDays         int       `json:"bonus_days"`
}

// ValidatePromo is the read-only preview check used by the standalone
// POST /billing/promo/validate endpoint: it tells the client what a code
// would do, without granting or reserving anything. Because nothing is
// redeemed here, a plain (non-locking, non-transactional) read is correct —
// there is no decision to protect against a concurrent redeemer. The actual
// redemption path (StartCheckout, in checkout.go) does NOT call this: it
// re-validates under a row lock via validatePromoLocked so the limit checks
// below can't race. See that function's doc comment for why.
func (s Service) ValidatePromo(ctx context.Context, profileID uuid.UUID, code string, tariffCode string) (ValidatePromoResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return ValidatePromoResult{}, ErrPromoNotFound
	}

	promo, err := s.Q.GetPromoCodeByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ValidatePromoResult{}, ErrPromoNotFound
		}
		return ValidatePromoResult{}, fmt.Errorf("get promo code: %w", err)
	}
	return s.evaluatePromo(ctx, promo, profileID, tariffCode)
}

// validatePromoLocked is ValidatePromo's sibling for the actual redemption
// path: it fetches the promo_code row via GetPromoCodeByCodeForUpdate
// (SELECT ... FOR UPDATE), which locks the row for the rest of the caller's
// transaction. Called from StartCheckout with a tx-bound Service (s.Q
// backed by a pgx.Tx), so two concurrent StartCheckout calls for the same
// promo code serialize on this row: the second one's FOR UPDATE blocks
// until the first commits or rolls back, and then re-reads the
// post-commit redemption counts — closing exactly the race that plain
// SELECT COUNT(*) (no lock, no tx) left open.
func (s Service) validatePromoLocked(ctx context.Context, profileID uuid.UUID, code, tariffCode string) (ValidatePromoResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return ValidatePromoResult{}, ErrPromoNotFound
	}

	promo, err := s.Q.GetPromoCodeByCodeForUpdate(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ValidatePromoResult{}, ErrPromoNotFound
		}
		return ValidatePromoResult{}, fmt.Errorf("get promo code for update: %w", err)
	}
	return s.evaluatePromo(ctx, promo, profileID, tariffCode)
}

// evaluatePromo is the validity/limit/discount logic shared by ValidatePromo
// and validatePromoLocked — identical either way, the only difference is
// whether the caller already holds a row lock on promo.
func (s Service) evaluatePromo(ctx context.Context, promo sqlc.PromoCode, profileID uuid.UUID, tariffCode string) (ValidatePromoResult, error) {
	now := time.Now()
	if promo.ValidFrom.Valid && now.Before(promo.ValidFrom.Time) {
		return ValidatePromoResult{}, ErrPromoNotStarted
	}
	if promo.ValidTo.Valid && now.After(promo.ValidTo.Time) {
		return ValidatePromoResult{}, ErrPromoExpired
	}

	userCount, err := s.Q.CountUserPromoRedemptions(ctx, sqlc.CountUserPromoRedemptionsParams{
		PromoCodeID: promo.ID,
		ProfileID:   profileID,
	})
	if err != nil {
		return ValidatePromoResult{}, fmt.Errorf("count user promo redemptions: %w", err)
	}
	if userCount >= promo.PerUserLimit {
		return ValidatePromoResult{}, ErrPromoUserLimitReached
	}

	if promo.MaxUses.Valid {
		count, err := s.Q.CountPromoRedemptions(ctx, promo.ID)
		if err != nil {
			return ValidatePromoResult{}, fmt.Errorf("count promo redemptions: %w", err)
		}
		if count >= promo.MaxUses.Int32 {
			return ValidatePromoResult{}, ErrPromoLimitReached
		}
	}

	tariff, err := s.Q.GetActiveTariffByCode(ctx, tariffCode)
	if err != nil {
		return ValidatePromoResult{}, fmt.Errorf("tariff %q: %w", tariffCode, err)
	}

	var discount int64
	var bonusDays int
	origAmount := tariff.PriceUzs

	switch promo.Kind {
	case "percent":
		discount = origAmount * int64(promo.Value) / 100
		if discount > origAmount {
			discount = origAmount
		}
	case "fixed":
		discount = int64(promo.Value)
		if discount > origAmount {
			discount = origAmount
		}
	case "days":
		discount = origAmount // 100% discount, 0 UZS
		bonusDays = int(promo.Value)
	}

	finalAmount := origAmount - discount
	if finalAmount < 0 {
		finalAmount = 0
	}

	return ValidatePromoResult{
		PromoID:           promo.ID,
		Code:              promo.Code,
		Kind:              promo.Kind,
		Value:             int(promo.Value),
		DiscountUzs:       discount,
		OriginalAmountUzs: origAmount,
		FinalAmountUzs:    finalAmount,
		BonusDays:         bonusDays,
	}, nil
}
