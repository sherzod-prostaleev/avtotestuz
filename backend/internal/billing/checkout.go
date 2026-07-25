package billing

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"avtotest.uz/backend/internal/db/sqlc"
)

// BuildPaymeURL builds the base64 GET checkout URL for Payme. amountUZS is in
// so'm; Payme wants tiyin (×100). Empty locale/returnURL segments are omitted.
func BuildPaymeURL(host, merchantID, orderID string, amountUZS int64, locale, returnURL string) string {
	parts := []string{
		"m=" + merchantID,
		"ac.order_id=" + orderID,
		fmt.Sprintf("a=%d", amountUZS*100),
	}
	if locale != "" {
		parts = append(parts, "l="+locale)
	}
	if returnURL != "" {
		parts = append(parts, "c="+returnURL)
	}
	raw := strings.Join(parts, ";")
	return strings.TrimRight(host, "/") + "/" + base64.StdEncoding.EncodeToString([]byte(raw))
}

// BuildClickURL builds the GET checkout URL for Click. amountUZS is in so'm
// (no tiyin conversion, unlike Payme). returnURL is omitted if empty.
func BuildClickURL(serviceID, merchantID, orderID string, amountUZS int64, returnURL string) string {
	v := url.Values{}
	v.Set("service_id", serviceID)
	v.Set("merchant_id", merchantID)
	v.Set("amount", strconv.FormatInt(amountUZS, 10))
	v.Set("transaction_param", orderID)
	if returnURL != "" {
		v.Set("return_url", returnURL)
	}
	return "https://my.click.uz/services/pay?" + v.Encode()
}

type CheckoutResult struct {
	PaymentID   uuid.UUID `json:"payment_id"`
	CheckoutURL string    `json:"checkout_url"`
	Free        bool      `json:"free,omitempty"`
}

// CheckoutConfig carries the provider-specific merchant identifiers needed
// to build a checkout URL. Populated by the caller (handlers.go) from
// config, not read from config directly here.
type CheckoutConfig struct {
	PaymeMerchantID   string
	PaymeCheckoutHost string
	ClickServiceID    string
	ClickMerchantID   string
}

// checkoutPendingReturnURL is where Payme/Click send the user after they finish
// on the provider side. Built from PUBLIC_BASE_URL + locale so checkout clients
// cannot supply an open-redirect target.
func (s Service) checkoutPendingReturnURL(locale string) string {
	return fmt.Sprintf("%s/%s/checkout/pending", s.publicBaseURL(), locale)
}

// StartCheckout creates a 'created' payment for the tariff and returns the
// checkout URL for the requested provider ("payme" or "click"; anything
// else falls back to Payme). Amount is the final price after applying
// promoCode if present.
//
// The whole read-decide-write sequence — promo re-validation, payment
// creation, and (for a zero-amount result) marking it paid and granting
// entitlement — runs inside a single DB transaction, with the promo_code
// row locked for its duration via validatePromoLocked (SELECT ... FOR
// UPDATE). This mirrors the pattern payme.performTransaction and
// click.confirmAndGrant already use for their own double-grant races
// (lock the row that decides the outcome, decide and write inside one tx):
// a reviewer proved the previous plain-SELECT, no-transaction ValidatePromo
// call here let 10 concurrent StartCheckout requests all read the same
// stale "0 used" count and all pass a max_uses=1 check, redeeming a
// one-time promo 10/10 times. Locking the promo_code row forces concurrent
// callers to serialize: the second one's FOR UPDATE blocks until the first
// commits (or rolls back), then re-reads the post-commit count and
// correctly sees the limit reached.
//
// The zero-amount branch's MarkPaymentPaid + GrantDays + CreatePromoRedemption
// stay inline here rather than being reused via entitlement.ProcessPaymentGrant
// (used by payme/click's own paid-completion path): ProcessPaymentGrant
// grants with source="purchase" and no note, but a zero-amount *promo*
// checkout is intentionally recorded as source="promo" with a distinguishing
// note — that's an existing, meaningful distinction (e.g. for future
// revenue/BI queries separating "paid" from "comped via promo") this fix
// has no reason to erase. What matters for money-safety is that the three
// statements commit or fail together, which putting them in this same tx
// achieves either way.
//
// For a non-zero-amount checkout this lock is NOT the limit-enforcement
// point, and must not be mistaken for one: the promo_redemption row that
// max_uses / per_user_limit are counted from is only written when the payment
// completes, in entitlement.ProcessPaymentGrant. Until then the counts read
// here stay at their pre-checkout value and a 'created' payment never
// expires, so this check alone was bypassable with no concurrency at all —
// an auditor banked five sequential discounted checkouts on a
// max_uses=1, per_user_limit=1 code and completed all five. Enforcement
// therefore lives in ProcessPaymentGrant, which re-validates under the same
// promo_code lock and pro-rates the grant when the code is no longer
// redeemable; see its doc comment. What this lock does buy is a correct,
// immediate answer for the common case (so a legitimate customer is told
// "limit reached" at checkout rather than after paying) and full protection
// for the zero-amount branch below, where the redemption row IS written here.
func (s Service) StartCheckout(ctx context.Context, profileID uuid.UUID, tariffCode, provider string, cfg CheckoutConfig, locale, returnURL, promoCode string) (CheckoutResult, error) {
	tariff, err := s.Q.GetActiveTariffByCode(ctx, tariffCode)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("tariff %q: %w", tariffCode, err)
	}

	if s.Pool == nil {
		return CheckoutResult{}, fmt.Errorf("billing: StartCheckout requires Service.Pool to be set")
	}

	dbtx, err := s.Pool.Begin(ctx)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = dbtx.Rollback(ctx) }()
	q := sqlc.New(dbtx)
	txSvc := Service{Q: q}

	amount := tariff.PriceUzs
	var promoID uuid.NullUUID
	var bonusDays int

	if promoCode != "" {
		valRes, err := txSvc.validatePromoLocked(ctx, profileID, promoCode, tariffCode)
		if err != nil {
			return CheckoutResult{}, fmt.Errorf("validate promo: %w", err)
		}
		amount = valRes.FinalAmountUzs
		promoID = uuid.NullUUID{UUID: valRes.PromoID, Valid: true}
		bonusDays = valRes.BonusDays
	}

	paymentID, err := q.CreatePayment(ctx, sqlc.CreatePaymentParams{
		ProfileID:      profileID,
		TariffID:       tariff.ID,
		AmountUzs:      amount,
		Provider:       provider,
		IdempotencyKey: uuid.NewString(),
		PromoCodeID:    promoID,
		// Frozen here on purpose: the tariff row is mutable and this payment
		// may not complete for an unbounded time, so the completion path must
		// grant what was sold now, not what the tariff says later.
		TariffDaysSnapshot:     tariff.Days,
		TariffPriceUzsSnapshot: tariff.PriceUzs,
	})
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("create payment: %w", err)
	}

	if amount == 0 {
		if err := q.MarkPaymentPaid(ctx, paymentID); err != nil {
			return CheckoutResult{}, fmt.Errorf("mark paid: %w", err)
		}
		grantedDays := int(tariff.Days) + bonusDays
		if _, err := txSvc.GrantDays(ctx, profileID, grantedDays, "promo", "zero-amount promo checkout", uuid.NullUUID{}); err != nil {
			return CheckoutResult{}, fmt.Errorf("grant days: %w", err)
		}
		if promoID.Valid {
			if err := q.CreatePromoRedemption(ctx, sqlc.CreatePromoRedemptionParams{
				PromoCodeID: promoID.UUID,
				ProfileID:   profileID,
				PaymentID:   uuid.NullUUID{UUID: paymentID, Valid: true},
			}); err != nil {
				if isUniqueViolation(err) {
					// promo_redemption_one_per_payment (migration 0016):
					// not expected to be reachable given the promo_code
					// row lock held since validatePromoLocked above
					// (paymentID is freshly minted by CreatePayment in this
					// same tx, so no prior redemption for this exact
					// payment can exist) — but if some future change ever
					// hits it, report the same domain error a genuine
					// concurrent over-redemption would, not an opaque
					// internal error.
					return CheckoutResult{}, ErrPromoLimitReached
				}
				return CheckoutResult{}, fmt.Errorf("create promo redemption: %w", err)
			}
		}
		if err := dbtx.Commit(ctx); err != nil {
			return CheckoutResult{}, fmt.Errorf("commit: %w", err)
		}
		return CheckoutResult{
			PaymentID:   paymentID,
			CheckoutURL: "",
			Free:        true,
		}, nil
	}

	if err := dbtx.Commit(ctx); err != nil {
		return CheckoutResult{}, fmt.Errorf("commit: %w", err)
	}

	var checkoutURL string
	switch provider {
	case "click":
		checkoutURL = BuildClickURL(cfg.ClickServiceID, cfg.ClickMerchantID, paymentID.String(), amount, returnURL)
	default:
		checkoutURL = BuildPaymeURL(cfg.PaymeCheckoutHost, cfg.PaymeMerchantID, paymentID.String(), amount, locale, returnURL)
	}
	return CheckoutResult{
		PaymentID:   paymentID,
		CheckoutURL: checkoutURL,
	}, nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505) — e.g. the promo_redemption_one_per_payment
// partial unique index (migration 0016). Duplicated from
// payme.isUniqueViolation / auth.isUniqueViolation rather than imported:
// billing has no other reason to depend on either package.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
