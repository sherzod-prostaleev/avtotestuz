// Package b2b implements the school licence + classroom station model.
package b2b

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrForbidden is returned when the caller lacks the rights to act on an org.
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound is a missing org, license or station.
	ErrNotFound = errors.New("not found")
	// ErrConflict covers already-bound and already-used cases.
	ErrConflict = errors.New("conflict")
	// ErrInvalid is bad input (code, key, hwid hash, status).
	ErrInvalid = errors.New("invalid")
)

// Store reads/writes b2b_org* for the classroom station model.
type Store struct {
	Pool *pgxpool.Pool
}

// OrgStats aggregates seat usage for the admin dashboard.
type OrgStats struct {
	OrgID               uuid.UUID  `json:"org_id"`
	ActiveSeats         int64      `json:"active_seats"`
	SeatsUsed           int64      `json:"seats_used"`
	SeatsRemaining      int64      `json:"seats_remaining"`
	LicenseExpiringSoon bool       `json:"license_expiring_soon"`
	LicenseEndsAt       *time.Time `json:"license_ends_at,omitempty"`
}

// OrgStats computes classroom seat aggregates for the admin dashboard. There
// is no membership left to aggregate: seat usage is entirely a function of
// active licenses and bound stations.
func (s Store) OrgStats(ctx context.Context, orgID uuid.UUID) (OrgStats, error) {
	var out OrgStats
	out.OrgID = orgID
	if err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(seats), 0) FROM b2b_org_license
		WHERE org_id = $1 AND starts_at <= now() AND ends_at > now()`, orgID).Scan(&out.ActiveSeats); err != nil {
		return out, err
	}
	used, err := s.CountActiveStations(ctx, orgID)
	if err != nil {
		return out, err
	}
	out.SeatsUsed = used
	out.SeatsRemaining = out.ActiveSeats - used
	if out.SeatsRemaining < 0 {
		out.SeatsRemaining = 0
	}
	soon, endsAt, err := s.LicenseExpiringSoon(ctx, orgID, 14)
	if err != nil {
		return out, err
	}
	out.LicenseExpiringSoon = soon
	if endsAt == nil {
		endsAt, err = s.LicenseEndsAt(ctx, orgID)
		if err != nil {
			return out, err
		}
	}
	out.LicenseEndsAt = endsAt
	return out, nil
}
