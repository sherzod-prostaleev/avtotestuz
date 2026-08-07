package b2b

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// StationRow is a bound classroom PC.
type StationRow struct {
	ID          uuid.UUID `json:"id"`
	OrgID       uuid.UUID `json:"org_id"`
	Label       string    `json:"label"`
	Status      string    `json:"status"`
	ActivatedAt time.Time `json:"activated_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	ActivatedBy string    `json:"activated_by"`
}

// ErrSeatsExhausted means active stations already fill license seats. This is
// distinct from ErrCodeExhausted: a license can shrink after a code is
// minted, so the two counters are checked independently and need different
// user-facing messages ("your school has no free seats" vs. "ask your
// admin to revoke a station or rotate the key").
var ErrSeatsExhausted = errors.New("seats exhausted")

// ErrCodeExhausted means the enroll code has hit its own max_uses, separate
// from the org's live seat count.
var ErrCodeExhausted = errors.New("enroll code exhausted")

// ErrOrgSuspended means the org cannot grant station VIP.
var ErrOrgSuspended = errors.New("org suspended")

// ErrNoLicense means no active classroom license window.
var ErrNoLicense = errors.New("no active license")

// ErrInstallerKeyRotatedNoSeats means RotateInstallerKey ran its emergency
// stop -- the live installer key is now revoked, unconditionally -- but
// could not mint a replacement because the org has no free seat. Deliberately
// distinct from ErrSeatsExhausted: that one means "nothing happened", this one
// means "the leaked key is dead, but there is nothing to hand out until a
// seat frees up". Collapsing the two would hide from the caller (and the
// admin who just clicked rotate) that the emergency stop actually took
// effect.
var ErrInstallerKeyRotatedNoSeats = errors.New("installer key rotated: no free seats for a replacement")

// CountActiveStations returns bound active stations for an org.
func (s Store) CountActiveStations(ctx context.Context, orgID uuid.UUID) (int64, error) {
	var n int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM b2b_station
		WHERE org_id = $1 AND status = 'active'`, orgID).Scan(&n)
	return n, err
}

// LicenseEndsAt returns the latest active license end for an org (UTC).
func (s Store) LicenseEndsAt(ctx context.Context, orgID uuid.UUID) (*time.Time, error) {
	var ends time.Time
	err := s.Pool.QueryRow(ctx, `
		SELECT MAX(ends_at) FROM b2b_org_license
		WHERE org_id=$1 AND starts_at <= now() AND ends_at > now()`, orgID).Scan(&ends)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if ends.IsZero() {
		return nil, nil
	}
	t := ends.UTC()
	return &t, nil
}

// ListStations returns stations for an org.
func (s Store) ListStations(ctx context.Context, orgID uuid.UUID) ([]StationRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, org_id, label, status, activated_at, last_seen_at, activated_by
		FROM b2b_station WHERE org_id = $1
		ORDER BY
		  CASE status WHEN 'active' THEN 0 ELSE 1 END,
		  activated_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]StationRow, 0)
	for rows.Next() {
		var row StationRow
		if err := rows.Scan(&row.ID, &row.OrgID, &row.Label, &row.Status,
			&row.ActivatedAt, &row.LastSeenAt, &row.ActivatedBy); err != nil {
			return nil, err
		}
		row.ActivatedAt = row.ActivatedAt.UTC()
		row.LastSeenAt = row.LastSeenAt.UTC()
		out = append(out, row)
	}
	return out, rows.Err()
}

// RevokeStation marks a station revoked (frees a seat).
func (s Store) RevokeStation(ctx context.Context, orgID, stationID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE b2b_station SET status = 'revoked'
		WHERE id = $1 AND org_id = $2 AND status = 'active'`, stationID, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteStation removes a station row outright, together with the shadow
// profile it practised under.
//
// Revoking is the reversible half: the row stays, the seat is freed, and the
// PC's history stays queryable. Deleting is for a machine that is gone --
// sold, reimaged, or a test PC nobody wants in the list forever -- so it also
// takes the profile, and with it every session, saved question, memory row,
// mastery row and streak that belongs to that PC (all of those cascade from
// profile). Leaving the profile behind would leave a row no admin screen ever
// shows, holding practice history for a machine that no longer exists.
//
// The delete is scoped by org_id for the same reason RevokeStation is: without
// it any school's admin call could reach into another school's station by id.
//
// The profile delete is guarded on kind = 'station'. b2b_station.station_profile_id
// is ON DELETE SET NULL and nothing today can point it at a learner, but this
// query is the one place in the codebase that deletes a profile as a side
// effect of deleting something else, and a learner's account is not something
// to lose to a column that turned out to be wrong.
func (s Store) DeleteStation(ctx context.Context, orgID, stationID uuid.UUID) (label string, err error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var profileID *uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT label, station_profile_id FROM b2b_station
		WHERE id = $1 AND org_id = $2
		FOR UPDATE`, stationID, orgID,
	).Scan(&label, &profileID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM b2b_station WHERE id = $1 AND org_id = $2`, stationID, orgID); err != nil {
		return "", err
	}
	if profileID != nil {
		if _, err := tx.Exec(ctx,
			`DELETE FROM profile WHERE id = $1 AND kind = 'station'`, *profileID); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return label, nil
}

// SetOrgStatus updates org status (admin).
func (s Store) SetOrgStatus(ctx context.Context, orgID uuid.UUID, status string) error {
	status = strings.TrimSpace(status)
	if status != "active" && status != "suspended" {
		return fmt.Errorf("%w: status", ErrInvalid)
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE b2b_org SET status = $2 WHERE id = $1`, orgID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ActiveStationVIP implements billing.StationVIPChecker: the station must be
// active, under a non-suspended org with a live license. The station id comes
// from a verified JWT (see stationctx), never from a request header.
func (s Store) ActiveStationVIP(ctx context.Context, stationID uuid.UUID) (bool, *time.Time, error) {
	if stationID == uuid.Nil {
		return false, nil, nil
	}
	var ends time.Time
	err := s.Pool.QueryRow(ctx, `
		SELECT MAX(l.ends_at)
		FROM b2b_station s
		JOIN b2b_org o ON o.id = s.org_id AND o.status = 'active'
		JOIN b2b_org_license l ON l.org_id = s.org_id
		  AND l.starts_at <= now() AND l.ends_at > now()
		WHERE s.id = $1 AND s.status = 'active'
		GROUP BY s.org_id`, stationID).Scan(&ends)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil, nil
		}
		return false, nil, err
	}
	_, _ = s.Pool.Exec(ctx, `
		UPDATE b2b_station SET last_seen_at = now() WHERE id = $1`, stationID)
	t := ends.UTC()
	return true, &t, nil
}

// LicenseExpiringSoon reports if any active license ends within days.
func (s Store) LicenseExpiringSoon(ctx context.Context, orgID uuid.UUID, withinDays int) (bool, *time.Time, error) {
	if withinDays <= 0 {
		withinDays = 14
	}
	var ends pgtype.Timestamptz
	err := s.Pool.QueryRow(ctx, `
		SELECT MIN(ends_at) FROM b2b_org_license
		WHERE org_id=$1 AND starts_at <= now() AND ends_at > now()
		  AND ends_at <= now() + ($2 * interval '1 day')`,
		orgID, withinDays).Scan(&ends)
	if err != nil {
		return false, nil, err
	}
	if !ends.Valid {
		return false, nil, nil
	}
	t := ends.Time.UTC()
	return true, &t, nil
}
