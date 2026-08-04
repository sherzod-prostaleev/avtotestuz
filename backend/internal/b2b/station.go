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

// ErrSeatsExhausted means active stations already fill license seats.
var ErrSeatsExhausted = errors.New("seats exhausted")

// ErrOrgSuspended means the org cannot grant station VIP.
var ErrOrgSuspended = errors.New("org suspended")

// ErrNoLicense means no active classroom license window.
var ErrNoLicense = errors.New("no active license")

// CountActiveStations returns bound active stations for an org.
func (s Store) CountActiveStations(ctx context.Context, orgID uuid.UUID) (int64, error) {
	var n int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM b2b_station
		WHERE org_id = $1 AND status = 'active'`, orgID).Scan(&n)
	return n, err
}

// ActiveHomeSeats returns sum of home_seats on currently active licenses.
func (s Store) ActiveHomeSeats(ctx context.Context, orgID uuid.UUID) (int64, error) {
	var n int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(home_seats), 0) FROM b2b_org_license
		WHERE org_id=$1 AND starts_at <= now() AND ends_at > now()`, orgID).Scan(&n)
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

// ListStationsAsTeacher lists stations if caller is owner/teacher.
func (s Store) ListStationsAsTeacher(ctx context.Context, actorID, orgID uuid.UUID) ([]StationRow, error) {
	if _, err := s.teacherRole(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	return s.ListStations(ctx, orgID)
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

// RevokeStationAsTeacher requires owner/teacher.
func (s Store) RevokeStationAsTeacher(ctx context.Context, actorID, orgID, stationID uuid.UUID) error {
	if _, err := s.teacherRole(ctx, actorID, orgID); err != nil {
		return err
	}
	return s.RevokeStation(ctx, orgID, stationID)
}

// RenameStationAsTeacher updates label.
func (s Store) RenameStationAsTeacher(ctx context.Context, actorID, orgID, stationID uuid.UUID, label string) (StationRow, error) {
	if _, err := s.teacherRole(ctx, actorID, orgID); err != nil {
		return StationRow{}, err
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return StationRow{}, fmt.Errorf("%w: label required", ErrInvalid)
	}
	var row StationRow
	err := s.Pool.QueryRow(ctx, `
		UPDATE b2b_station SET label = $3
		WHERE id = $1 AND org_id = $2
		RETURNING id, org_id, label, status, activated_at, last_seen_at, activated_by`,
		stationID, orgID, label,
	).Scan(&row.ID, &row.OrgID, &row.Label, &row.Status,
		&row.ActivatedAt, &row.LastSeenAt, &row.ActivatedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StationRow{}, ErrNotFound
		}
		return StationRow{}, err
	}
	row.ActivatedAt = row.ActivatedAt.UTC()
	row.LastSeenAt = row.LastSeenAt.UTC()
	return row, nil
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
