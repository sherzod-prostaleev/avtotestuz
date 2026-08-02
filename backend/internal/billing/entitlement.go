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
	"avtotest.uz/backend/internal/devicefp"
)

func stationFingerprint(ctx context.Context) string {
	return devicefp.FromContext(ctx)
}

// Service is billing's core, reused both non-transactionally (Q backed
// directly by the pool — most reads, and writes with no cross-statement
// invariant to protect) and transactionally (Q backed by a pgx.Tx, built
// fresh per-transaction — see payme.performTransaction, click.confirmAndGrant,
// and StartCheckout for the pattern: sqlc.New(tx) then Service{Q: q}).
// Pool is only needed by methods that must themselves open a transaction
// (currently just StartCheckout, for its row-locked promo redemption) — it
// is nil, harmlessly, for the tx-bound Service values those methods never
// call.
// StationVIPChecker grants classroom VIP when the request device is a bound
// school station under a live, non-suspended org license.
type StationVIPChecker interface {
	ActiveStationVIP(ctx context.Context, fingerprint string) (active bool, until *time.Time, err error)
}

type Service struct {
	Q    *sqlc.Queries
	Pool *pgxpool.Pool

	// StationVIP is optional; when set, Status also accepts device-bound B2B VIP.
	StationVIP StationVIPChecker

	// PublicBaseURL is the frontend origin used to build shareable referral
	// invite links. Optional: the tx-bound Service values built inside webhook
	// handlers never generate links, and publicBaseURL() falls back to the dev
	// frontend so a missing value degrades to a working local link rather than
	// a link to nowhere.
	PublicBaseURL string

	// Secret is the deployment's JWT secret. It is only the FALLBACK data key
	// now (see DataSecret); nothing in billing signs tokens with it.
	Secret []byte

	// DataSecret is the dedicated at-rest key (DATA_ENCRYPTION_KEY) from
	// which the AES-GCM KEKs for stored card PANs and the manual-pay Telegram
	// userbot credentials are derived. Empty falls back to Secret, which is
	// what sealed every ciphertext written before the two keys were split —
	// that fallback, not a migration, is what keeps those rows readable.
	// Optional for tx-bound Service values that touch neither.
	DataSecret []byte
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

// Status reports whether profileID currently has an active entitlement, or
// (when StationVIP is configured) whether the request device fingerprint is a
// bound classroom station with a live org license.
func (s Service) Status(ctx context.Context, profileID uuid.UUID) (active bool, until *time.Time, err error) {
	ends, err := s.Q.ActiveEntitlementEnd(ctx, profileID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, nil, err
		}
	} else {
		t := ends.Time
		return true, &t, nil
	}
	if s.StationVIP == nil {
		return false, nil, nil
	}
	fp := stationFingerprint(ctx)
	if fp == "" {
		return false, nil, nil
	}
	return s.StationVIP.ActiveStationVIP(ctx, fp)
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
	return s.GrantDaysForPayment(ctx, profileID, days, source, note, by, uuid.NullUUID{})
}

// GrantDaysForPayment is GrantDays with an optional payment_id link so a later
// provider refund can clamp that exact entitlement row.
func (s Service) GrantDaysForPayment(ctx context.Context, profileID uuid.UUID, days int, source, note string, by, paymentID uuid.NullUUID) (time.Time, error) {
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
		PaymentID: paymentID,
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
			note = formatPromoProratedNote(promo.Code, promoCauseCode(redeemErr), days, int(payment.TariffDays), payment.AmountUzs, payment.TariffPriceUzs)
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

	if _, err := s.GrantDaysForPayment(ctx, payment.ProfileID, days, source, note, uuid.NullUUID{}, uuid.NullUUID{UUID: payment.ID, Valid: true}); err != nil {
		return err
	}
	if err := s.processReferralRewardOnPayment(ctx, payment.ProfileID, payment); err != nil {
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

// promoProratedNotePrefix marks entitlement.note rows written when a promo
// could not be honored at payment completion and days were scaled to the
// amount actually paid. Learner APIs parse this for checkout UX; the pipe
// fields remain for support reconciliation.
const promoProratedNotePrefix = "promo_prorated|"

func promoCauseCode(err error) string {
	switch {
	case errors.Is(err, ErrPromoLimitReached), errors.Is(err, ErrPromoUserLimitReached):
		return "promo_limit_reached"
	case errors.Is(err, ErrPromoExpired):
		return "promo_expired"
	case errors.Is(err, ErrPromoNotStarted):
		return "promo_not_started"
	case errors.Is(err, ErrPromoNotFound):
		return "promo_unavailable"
	default:
		return "promo_unavailable"
	}
}

func formatPromoProratedNote(code, cause string, granted, tariffDays int, paid, list int64) string {
	return fmt.Sprintf(
		"%scode=%s|cause=%s|granted=%d|tariff=%d|paid=%d|list=%d",
		promoProratedNotePrefix, code, cause, granted, tariffDays, paid, list,
	)
}

// ProrationInfo is the learner-facing summary of a pro-rated purchase grant.
type ProrationInfo struct {
	Applied     bool   `json:"applied"`
	GrantedDays int    `json:"granted_days"`
	TariffDays  int    `json:"tariff_days"`
	Reason      string `json:"reason"`
}

// ParseProrationNote extracts ProrationInfo from an entitlement note, if any.
func ParseProrationNote(note string) (ProrationInfo, bool) {
	if note == "" || len(note) < len(promoProratedNotePrefix) || note[:len(promoProratedNotePrefix)] != promoProratedNotePrefix {
		return ProrationInfo{}, false
	}
	info := ProrationInfo{Applied: true, Reason: "promo_unavailable"}
	for _, part := range splitNoteFields(note[len(promoProratedNotePrefix):]) {
		key, val, ok := cutField(part)
		if !ok {
			continue
		}
		switch key {
		case "cause":
			info.Reason = val
		case "granted":
			_, _ = fmt.Sscanf(val, "%d", &info.GrantedDays)
		case "tariff":
			_, _ = fmt.Sscanf(val, "%d", &info.TariffDays)
		}
	}
	if info.GrantedDays < 1 || info.TariffDays < 1 {
		return ProrationInfo{}, false
	}
	return info, true
}

func splitNoteFields(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '|' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

func cutField(s string) (key, val string, ok bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}

// LatestPurchaseProration returns proration details for the newest purchase
// grant, if that grant was pro-rated.
func (s Service) LatestPurchaseProration(ctx context.Context, profileID uuid.UUID) (*ProrationInfo, error) {
	row, err := s.Q.GetLatestPurchaseEntitlement(ctx, profileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	info, ok := ParseProrationNote(row.Note)
	if !ok {
		return nil, nil
	}
	return &info, nil
}

// RevokeEntitlementForPayment clamps VIP grant(s) linked to a refunded
// payment so they no longer extend past now. Idempotent: missing grant or an
// already-expired row is a no-op. Also claws back referral commission when
// the referrer still has available balance.
func (s Service) RevokeEntitlementForPayment(ctx context.Context, paymentID uuid.UUID) error {
	ent, err := s.Q.GetEntitlementByPaymentID(ctx, uuid.NullUUID{UUID: paymentID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if err := s.ClawbackReferralCommissionOnRefund(ctx, paymentID); err != nil {
				return fmt.Errorf("referral commission clawback: %w", err)
			}
			return nil
		}
		return fmt.Errorf("lookup entitlement for payment %s: %w", paymentID, err)
	}
	if _, err := s.Q.LockProfileForGrant(ctx, ent.ProfileID); err != nil {
		return fmt.Errorf("lock profile %s for revoke: %w", ent.ProfileID, err)
	}
	now := time.Now().UTC()
	// CHECK (ends_at > starts_at): collapse not-yet-started grants to a 1s stub.
	clampTo := now
	if ent.StartsAt.Valid && !clampTo.After(ent.StartsAt.Time) {
		clampTo = ent.StartsAt.Time.Add(time.Second)
	}
	if err := s.Q.ClampAllEntitlementsForPayment(ctx, sqlc.ClampAllEntitlementsForPaymentParams{
		PaymentID:  uuid.NullUUID{UUID: paymentID, Valid: true},
		ClampTo:    pgtype.Timestamptz{Time: clampTo, Valid: true},
		NoteSuffix: " | refunded",
	}); err != nil {
		return fmt.Errorf("clamp entitlements for payment %s: %w", paymentID, err)
	}
	if err := s.ClawbackReferralCommissionOnRefund(ctx, paymentID); err != nil {
		return fmt.Errorf("referral commission clawback: %w", err)
	}
	return nil
}
