// Package stationctx carries a verified B2B station id on the request
// context so entitlement checks can grant classroom VIP without a personal
// purchase.
//
// It replaces internal/devicefp, which read an attacker-controlled header.
// Nothing here parses input: the id is written only by auth.Required after a
// station JWT has been verified, so anything found on the context is already
// authenticated.
package stationctx

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey struct{}

// WithContext stores a verified station id on ctx. uuid.Nil is ignored so a
// zero value can never be mistaken for a bound station.
func WithContext(ctx context.Context, stationID uuid.UUID) context.Context {
	if stationID == uuid.Nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, stationID)
}

// FromContext returns the station id if the request was authenticated as a
// classroom station.
func FromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok
}
