package billing

import "math"

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
