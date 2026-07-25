// Package billing owns entitlement math: whether a profile currently has
// an active (paid or granted) pass, and stacking new grants on top.
package billing

import (
	"context"
	"errors"
	"fmt"
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

	// PublicBaseURL is the frontend origin used to build shareable referral
	// invite links. Optional: the tx-bound Service values built inside webhook
	// handlers never generate links, and publicBaseURL() falls back to the dev
	// frontend so a missing value degrades to a working local link rather than
	// a link to nowhere.
	PublicBaseURL string
}

// defaultPublicBaseURL matches config's PUBLIC_BASE_URL default so a Service
// constructed without one (tests, CLIs) still produces a link that resolves in
// a normal dev setup.
const defaultPublicBaseURL = "http://localhost:3000"

func (s Service) publicBaseURL() string {
	if s.PublicBaseURL != "" {
		return s.PublicBaseURL
	}
	return defaultPublicBaseURL
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
//
// The profile row is locked first (LockProfileForGrant). The stacking read
// (ActiveEntitlementEnd) and the write (InsertEntitlement) touch no common
// row, so under READ COMMITTED two concurrent grants for the same profile
// both read the same pre-existing end and both insert the same interval —
// the second grant's days are silently lost. An auditor demonstrated this
// with one referrer whose two referees paid simultaneously: both referrals
// were permanently marked 'rewarded' while the referrer received 7 days
// instead of 14. The same defect hits a customer whose two payments complete
// together, because the payme_transaction / click_transaction locks those
// paths take are disjoint and serialize nothing here.
//
// The lock only serializes when s.Q is bound to a pgx.Tx — outside a
// transaction each statement auto-commits and releases it immediately. Every
// caller that can race is tx-bound: StartCheckout's zero-amount branch, and
// ProcessPaymentGrant via payme.performTransaction / click.confirmAndGrant.
// cmd/grantvip (single manual admin invocation) opens one for the same reason.
//
// Lock-ordering note: ProcessPaymentGrant takes promo_code before profile,
// and a referral reward takes the referral row before the referrer's profile.
// Two mutual referrals paying at the exact same instant can therefore deadlock
// on (profile A, profile B) in opposite orders; Postgres detects it (40P01),
// aborts one transaction whole, and the provider retries — correctness is
// preserved because the losing transaction rolls back entirely.
func (s Service) GrantDays(ctx context.Context, profileID uuid.UUID, days int, source, note string, by uuid.NullUUID) (time.Time, error) {
	if _, err := s.Q.LockProfileForGrant(ctx, profileID); err != nil {
		return time.Time{}, fmt.Errorf("lock profile %s for grant: %w", profileID, err)
	}

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
//
// Promo limits are re-validated here, under a FOR UPDATE lock on the
// promo_code row, because StartCheckout's own lock cannot be the enforcement
// point for a paid checkout: the promo_redemption row that the max_uses /
// per_user_limit counts are derived from is not written until completion —
// this function. Until then those counts stay at their pre-checkout value, and
// a 'created' payment never expires, so an auditor bypassed a
// max_uses=1, per_user_limit=1 code with five *sequential* POST /me/checkout
// calls (no concurrency, no prior payment) and then completed all five.
//
// When the code turns out not to be redeemable at this point — limit reached
// by a genuine earlier redemption, deactivated, or expired between checkout
// and payment — refusing the grant outright is not an option, because the
// customer's money has already moved. Instead the grant is pro-rated to the
// fraction of the list price actually paid (proRatedDays) and no redemption
// row is written. That keeps the economics neutral: an abuser who banked N
// discounted checkouts receives exactly the days their money is worth at list
// price, so there is nothing to gain, while a legitimate customer who lost a
// genuine race still gets value proportional to what they paid rather than a
// failed webhook or free days. The reason is recorded in the entitlement note
// for support reconciliation.
//
// Both the term and the list price come from the payment's own tariff snapshot
// (migration 0019), never from the tariff table: the same unbounded
// checkout-to-completion window that made the promo bypass possible would
// otherwise let a tariff edit change what an in-flight purchase is worth, and
// would leave the pro-rating ratio dividing a frozen amount by a moved price.
func (s Service) ProcessPaymentGrant(ctx context.Context, paymentID uuid.UUID) error {
	payment, err := s.Q.GetPaymentForPayme(ctx, paymentID)
	if err != nil {
		return err
	}
	days := int(payment.TariffDays)
	source := "purchase"
	note := ""

	if payment.PromoCodeID.Valid {
		// FOR UPDATE, so a concurrent StartCheckout or ProcessPaymentGrant for
		// the same code blocks here and then re-reads post-commit counts.
		promo, err := s.Q.GetPromoCodeByIDForUpdate(ctx, payment.PromoCodeID.UUID)
		if err != nil {
			// Previously this error was swallowed and the redemption written
			// anyway; a promo attached to the payment must never silently
			// vanish from the decision.
			return fmt.Errorf("lock promo code %s: %w", payment.PromoCodeID.UUID, err)
		}

		if redeemErr := s.checkPromoRedeemable(ctx, promo, payment.ProfileID); redeemErr != nil {
			if !isPromoDomainError(redeemErr) {
				return fmt.Errorf("re-validate promo %s: %w", promo.Code, redeemErr)
			}
			days = proRatedDays(payment.AmountUzs, payment.TariffPriceUzs, int(payment.TariffDays))
			note = fmt.Sprintf(
				"promo %s not redeemable at completion (%s): granted %d of %d days pro-rata for %d of %d UZS paid",
				promo.Code, redeemErr, days, payment.TariffDays, payment.AmountUzs, payment.TariffPriceUzs,
			)
		} else {
			if promo.Kind == "days" {
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
	}

	if _, err := s.GrantDays(ctx, payment.ProfileID, days, source, note, uuid.NullUUID{}); err != nil {
		return err
	}
	if err := s.processReferralRewardOnPayment(ctx, payment.ProfileID); err != nil {
		return err
	}
	return nil
}

// proRatedDays scales tariffDays down to the fraction of listPriceUZS that
// paidUZS actually covers, floored. A payment covering the full list price (or
// more) gets the full term. The result is never below 1: this is only reached
// for a payment that already completed, so a customer whose promo was rejected
// must not end up with money spent and nothing granted — and entitlement's
// CHECK (ends_at > starts_at) would reject a zero-day grant outright.
func proRatedDays(paidUZS, listPriceUZS int64, tariffDays int) int {
	if listPriceUZS <= 0 || paidUZS >= listPriceUZS {
		return tariffDays
	}
	days := int(int64(tariffDays) * paidUZS / listPriceUZS)
	if days < 1 {
		days = 1
	}
	return days
}
