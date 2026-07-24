package billing

import (
	"context"
	"math"
)

// pricePerDay is the rounded per-day cost used for the "~X so'm/kun" framing.
func pricePerDay(priceUZS int64, days int32) int64 {
	if days <= 0 {
		return priceUZS
	}
	return int64(math.Round(float64(priceUZS) / float64(days)))
}

// discountPercent is the rounded savings vs the old price, or 0 when there is
// no old price or it is not higher than the current price.
func discountPercent(priceUZS int64, oldPriceUZS *int64) int32 {
	if oldPriceUZS == nil || *oldPriceUZS <= priceUZS {
		return 0
	}
	return int32(math.Round(float64(*oldPriceUZS-priceUZS) / float64(*oldPriceUZS) * 100))
}

type TariffDTO struct {
	Code            string  `json:"code"`
	Days            int32   `json:"days"`
	PriceUZS        int64   `json:"price_uzs"`
	OldPriceUZS     *int64  `json:"old_price_uzs"`
	PricePerDayUZS  int64   `json:"price_per_day_uzs"`
	DiscountPercent int32   `json:"discount_percent"`
	Badge           *string `json:"badge"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
}

// ListTariffs returns active tariffs for a locale (uz-Latn fallback) with the
// per-day price and discount computed for display.
func (s Service) ListTariffs(ctx context.Context, locale string) ([]TariffDTO, error) {
	rows, err := s.Q.ListActiveTariffs(ctx, locale)
	if err != nil {
		return nil, err
	}
	out := make([]TariffDTO, 0, len(rows))
	for _, r := range rows {
		var old *int64
		if r.OldPriceUzs.Valid {
			v := r.OldPriceUzs.Int64
			old = &v
		}
		var badge *string
		if r.Badge.Valid && r.Badge.String != "" {
			b := r.Badge.String
			badge = &b
		}
		out = append(out, TariffDTO{
			Code:            r.Code,
			Days:            r.Days,
			PriceUZS:        r.PriceUzs,
			OldPriceUZS:     old,
			PricePerDayUZS:  pricePerDay(r.PriceUzs, r.Days),
			DiscountPercent: discountPercent(r.PriceUzs, old),
			Badge:           badge,
			Name:            r.Name,
			Description:     r.Description,
		})
	}
	return out, nil
}
