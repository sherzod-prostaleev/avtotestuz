package billing

import (
	"context"
	"encoding/base64"
	"fmt"
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

type CheckoutResult struct {
	PaymentID   uuid.UUID `json:"payment_id"`
	CheckoutURL string    `json:"checkout_url"`
}

// StartCheckout creates a 'created' payment for the tariff and returns the
// Payme checkout URL. Amount is the tariff price (promo is applied in M2-05).
func (s Service) StartCheckout(ctx context.Context, profileID uuid.UUID, tariffCode, merchantID, host, locale, returnURL string) (CheckoutResult, error) {
	tariff, err := s.Q.GetActiveTariffByCode(ctx, tariffCode)
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("tariff %q: %w", tariffCode, err)
	}
	paymentID, err := s.Q.CreatePayment(ctx, sqlc.CreatePaymentParams{
		ProfileID:      profileID,
		TariffID:       tariff.ID,
		AmountUzs:      tariff.PriceUzs,
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		return CheckoutResult{}, fmt.Errorf("create payment: %w", err)
	}
	return CheckoutResult{
		PaymentID:   paymentID,
		CheckoutURL: BuildPaymeURL(host, merchantID, paymentID.String(), tariff.PriceUzs, locale, returnURL),
	}, nil
}
