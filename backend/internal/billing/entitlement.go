// Package billing owns entitlement math: whether a profile currently has
// an active (paid or granted) pass, and stacking new grants on top.
package billing

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/db/sqlc"
)

// Service is billing's core, reused both non-transactionally (Q backed
// directly by the pool — most reads, and writes with no cross-statement
// invariant to protect) and transactionally (Q backed by a pgx.Tx, built
// fresh per-transaction — see payme.performTransaction, click.confirmAndGrant,
// and StartCheckout for the pattern: sqlc.New(tx) then Service{Q: q}).
// Pool is only needed by methods that must themselves open a transaction
// (currently just StartCheckout, for its row-locked promo redemption) — it
// is nil, harmlessly, for the tx-bound Service values those methods never
// call.
type Service struct {
	Q    *sqlc.Queries
	Pool *pgxpool.Pool
}

// Status reports whether profileID currently has an active entitlement.
func (s Service) Status(ctx context.Context, profileID uuid.UUID) (active bool, until *time.Time, err error) {
	ends, err := s.Q.ActiveEntitlementEnd(ctx, profileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil, nil
		}
		return false, nil, err
	}
	t := ends.Time
	return true, &t, nil
}

// GrantDays adds `days` of entitlement, stacking: starts at
// max(now, current active end) so back-to-back grants extend, not overlap.
func (s Service) GrantDays(ctx context.Context, profileID uuid.UUID, days int, source, note string, by uuid.NullUUID) (time.Time, error) {
	start := time.Now()
	ends, err := s.Q.ActiveEntitlementEnd(ctx, profileID)
	switch {
	case err == nil && ends.Time.After(start):
		start = ends.Time
	case err != nil && !errors.Is(err, pgx.ErrNoRows):
		return time.Time{}, err
	}

	end := start.Add(time.Duration(days) * 24 * time.Hour)
	_, err = s.Q.InsertEntitlement(ctx, sqlc.InsertEntitlementParams{
		ProfileID: profileID,
		Source:    source,
		StartsAt:  pgtype.Timestamptz{Time: start, Valid: true},
		EndsAt:    pgtype.Timestamptz{Time: end, Valid: true},
		Note:      note,
		CreatedBy: by,
	})
	if err != nil {
		return time.Time{}, err
	}
	return end, nil
}

// ProcessPaymentGrant grants entitlement days for a paid payment, handling
// any associated promo code bonus days and promo redemption creation.
func (s Service) ProcessPaymentGrant(ctx context.Context, paymentID uuid.UUID) error {
	payment, err := s.Q.GetPaymentForPayme(ctx, paymentID)
	if err != nil {
		return err
	}
	days := int(payment.TariffDays)
	source := "purchase"
	if payment.PromoCodeID.Valid {
		promo, err := s.Q.GetPromoCodeByID(ctx, payment.PromoCodeID.UUID)
		if err == nil && promo.Kind == "days" {
			days += int(promo.Value)
		}
		if err := s.Q.CreatePromoRedemption(ctx, sqlc.CreatePromoRedemptionParams{
			PromoCodeID: payment.PromoCodeID.UUID,
			ProfileID:   payment.ProfileID,
			PaymentID:   uuid.NullUUID{UUID: payment.ID, Valid: true},
		}); err != nil {
			return err
		}
	}
	_, err = s.GrantDays(ctx, payment.ProfileID, days, source, "", uuid.NullUUID{})
	if err != nil {
		return err
	}
	if err := s.processReferralRewardOnPayment(ctx, payment.ProfileID); err != nil {
		return err
	}
	return nil
}
