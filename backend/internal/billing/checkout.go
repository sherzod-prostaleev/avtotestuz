package billing

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

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

// StartCheckout creates a 'created' payment for the tariff and returns the
// checkout URL for the requested provider ("payme" or "click"; anything
// else falls back to Payme). Amount is the final price after applying promoCode if present.
func (s Service) StartCheckout(ctx context.Context, profileID uuid.UUID, tariffCode, provider string, cfg CheckoutConfig, locale, returnURL, promoCode string) (CheckoutResult, error) {
	tariff, err := s.Q.GetActiveTariffByCode(ctx, tariffCode)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("tariff %q: %w", tariffCode, err)
	}
	amount := tariff.PriceUzs
	var promoID uuid.NullUUID
	var bonusDays int

	if promoCode != "" {
		valRes, err := s.ValidatePromo(ctx, profileID, promoCode, tariffCode)
		if err != nil {
			return CheckoutResult{}, fmt.Errorf("validate promo: %w", err)
		}
		amount = valRes.FinalAmountUzs
		promoID = uuid.NullUUID{UUID: valRes.PromoID, Valid: true}
		bonusDays = valRes.BonusDays
	}

	paymentID, err := s.Q.CreatePayment(ctx, sqlc.CreatePaymentParams{
		ProfileID:      profileID,
		TariffID:       tariff.ID,
		AmountUzs:      amount,
		Provider:       provider,
		IdempotencyKey: uuid.NewString(),
		PromoCodeID:    promoID,
	})
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("create payment: %w", err)
	}

	if amount == 0 {
		if err := s.Q.MarkPaymentPaid(ctx, paymentID); err != nil {
			return CheckoutResult{}, fmt.Errorf("mark paid: %w", err)
		}
		grantedDays := int(tariff.Days) + bonusDays
		if _, err := s.GrantDays(ctx, profileID, grantedDays, "promo", "zero-amount promo checkout", uuid.NullUUID{}); err != nil {
			return CheckoutResult{}, fmt.Errorf("grant days: %w", err)
		}
		if promoID.Valid {
			if err := s.Q.CreatePromoRedemption(ctx, sqlc.CreatePromoRedemptionParams{
				PromoCodeID: promoID.UUID,
				ProfileID:   profileID,
				PaymentID:   uuid.NullUUID{UUID: paymentID, Valid: true},
			}); err != nil {
				return CheckoutResult{}, fmt.Errorf("create promo redemption: %w", err)
			}
		}
		return CheckoutResult{
			PaymentID:   paymentID,
			CheckoutURL: "",
			Free:        true,
		}, nil
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
