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

	"avtotest.uz/backend/internal/db/sqlc"
)

type Service struct {
	Q *sqlc.Queries
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
