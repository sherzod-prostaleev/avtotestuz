package b2b

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/devicefp"
)

// StationRow is a bound classroom PC.
type StationRow struct {
	ID          uuid.UUID `json:"id"`
	OrgID       uuid.UUID `json:"org_id"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	Label       string    `json:"label"`
	Status      string    `json:"status"`
	ActivatedAt time.Time `json:"activated_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	ActivatedBy string    `json:"activated_by"`
}

// ActivateCodeRow is a one-time station activation code.
type ActivateCodeRow struct {
	ID        uuid.UUID  `json:"id"`
	OrgID     uuid.UUID  `json:"org_id"`
	Code      string     `json:"code"`
	Label     string     `json:"label"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// ErrSeatsExhausted means active stations already fill license seats.
var ErrSeatsExhausted = errors.New("seats exhausted")

// ErrOrgSuspended means the org cannot grant station VIP.
var ErrOrgSuspended = errors.New("org suspended")

// ErrNoLicense means no active classroom license window.
var ErrNoLicense = errors.New("no active license")

func newActivateCode() (string, error) {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	raw := strings.ToUpper(hex.EncodeToString(buf))
	// XXXXX-XXXXX style (10 hex chars).
	return raw[:5] + "-" + raw[5:], nil
}

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

// ActiveStationVIP implements billing.StationVIPChecker: fingerprint must be an
// active station under an active, non-suspended org with a live license.
func (s Store) ActiveStationVIP(ctx context.Context, fingerprint string) (bool, *time.Time, error) {
	fp := devicefp.Normalize(fingerprint)
	if fp == "" {
		return false, nil, nil
	}
	var orgID uuid.UUID
	var ends time.Time
	err := s.Pool.QueryRow(ctx, `
		SELECT s.org_id, MAX(l.ends_at)
		FROM b2b_station s
		JOIN b2b_org o ON o.id = s.org_id AND o.status = 'active'
		JOIN b2b_org_license l ON l.org_id = s.org_id
		  AND l.starts_at <= now() AND l.ends_at > now()
		WHERE s.fingerprint = $1 AND s.status = 'active'
		GROUP BY s.org_id`, fp).Scan(&orgID, &ends)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil, nil
		}
		return false, nil, err
	}
	_, _ = s.Pool.Exec(ctx, `
		UPDATE b2b_station SET last_seen_at = now()
		WHERE fingerprint = $1 AND status = 'active'`, fp)
	t := ends.UTC()
	return true, &t, nil
}

// CreateActivateCode issues a one-time code (admin or teacher/owner).
func (s Store) CreateActivateCode(ctx context.Context, orgID uuid.UUID, label, createdBy string, ttl time.Duration) (ActivateCodeRow, error) {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	if ttl > 30*24*time.Hour {
		ttl = 30 * 24 * time.Hour
	}
	var status string
	err := s.Pool.QueryRow(ctx, `SELECT status FROM b2b_org WHERE id=$1`, orgID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ActivateCodeRow{}, ErrNotFound
		}
		return ActivateCodeRow{}, err
	}
	if status != "active" {
		return ActivateCodeRow{}, ErrOrgSuspended
	}
	code, err := newActivateCode()
	if err != nil {
		return ActivateCodeRow{}, err
	}
	var row ActivateCodeRow
	var usedAt pgtype.Timestamptz
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO b2b_station_activate_code (org_id, code, label, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, org_id, code, label, expires_at, used_at, created_at`,
		orgID, code, strings.TrimSpace(label), time.Now().UTC().Add(ttl), createdBy,
	).Scan(&row.ID, &row.OrgID, &row.Code, &row.Label, &row.ExpiresAt, &usedAt, &row.CreatedAt)
	if err != nil {
		return ActivateCodeRow{}, err
	}
	if usedAt.Valid {
		t := usedAt.Time.UTC()
		row.UsedAt = &t
	}
	row.ExpiresAt = row.ExpiresAt.UTC()
	row.CreatedAt = row.CreatedAt.UTC()
	return row, nil
}

// CreateActivateCodeAsTeacher requires owner/teacher.
func (s Store) CreateActivateCodeAsTeacher(ctx context.Context, actorID, orgID uuid.UUID, label string, ttl time.Duration) (ActivateCodeRow, error) {
	if _, err := s.teacherRole(ctx, actorID, orgID); err != nil {
		return ActivateCodeRow{}, err
	}
	return s.CreateActivateCode(ctx, orgID, label, "profile:"+actorID.String(), ttl)
}

// ListStations returns stations for an org (fingerprint redacted for teacher UI optional).
func (s Store) ListStations(ctx context.Context, orgID uuid.UUID, includeFingerprint bool) ([]StationRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, org_id, fingerprint, label, status, activated_at, last_seen_at, activated_by
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
		if err := rows.Scan(&row.ID, &row.OrgID, &row.Fingerprint, &row.Label, &row.Status,
			&row.ActivatedAt, &row.LastSeenAt, &row.ActivatedBy); err != nil {
			return nil, err
		}
		row.ActivatedAt = row.ActivatedAt.UTC()
		row.LastSeenAt = row.LastSeenAt.UTC()
		if !includeFingerprint {
			row.Fingerprint = maskFingerprint(row.Fingerprint)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListStationsAsTeacher lists stations if caller is owner/teacher.
func (s Store) ListStationsAsTeacher(ctx context.Context, actorID, orgID uuid.UUID) ([]StationRow, error) {
	if _, err := s.teacherRole(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	return s.ListStations(ctx, orgID, false)
}

func maskFingerprint(fp string) string {
	fp = strings.TrimSpace(fp)
	if len(fp) <= 8 {
		return "****"
	}
	return fp[:4] + "…" + fp[len(fp)-4:]
}

// ActivateStation binds fingerprint using a one-time code.
func (s Store) ActivateStation(ctx context.Context, code, fingerprint, label, activatedBy string) (StationRow, error) {
	fp := devicefp.Normalize(fingerprint)
	if fp == "" {
		return StationRow{}, fmt.Errorf("%w: fingerprint required", ErrInvalid)
	}
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return StationRow{}, fmt.Errorf("%w: code required", ErrInvalid)
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return StationRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		codeID    uuid.UUID
		orgID     uuid.UUID
		codeLabel string
		expiresAt time.Time
		usedAt    *time.Time
		orgStatus string
	)
	err = tx.QueryRow(ctx, `
		SELECT c.id, c.org_id, c.label, c.expires_at, c.used_at, o.status
		FROM b2b_station_activate_code c
		JOIN b2b_org o ON o.id = c.org_id
		WHERE c.code = $1
		FOR UPDATE OF c`, code).Scan(&codeID, &orgID, &codeLabel, &expiresAt, &usedAt, &orgStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StationRow{}, ErrNotFound
		}
		return StationRow{}, err
	}
	if orgStatus != "active" {
		return StationRow{}, ErrOrgSuspended
	}
	if usedAt != nil {
		return StationRow{}, fmt.Errorf("%w: code already used", ErrConflict)
	}
	if !expiresAt.After(time.Now().UTC()) {
		return StationRow{}, fmt.Errorf("%w: code expired", ErrInvalid)
	}

	var seats int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(seats), 0) FROM b2b_org_license
		WHERE org_id=$1 AND starts_at <= now() AND ends_at > now()`, orgID).Scan(&seats); err != nil {
		return StationRow{}, err
	}
	if seats <= 0 {
		return StationRow{}, ErrNoLicense
	}
	var usedStations int64
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM b2b_station WHERE org_id=$1 AND status='active'`, orgID).Scan(&usedStations); err != nil {
		return StationRow{}, err
	}
	if usedStations >= seats {
		return StationRow{}, ErrSeatsExhausted
	}

	// If this fingerprint was previously active elsewhere, revoke it first.
	_, _ = tx.Exec(ctx, `
		UPDATE b2b_station SET status = 'revoked'
		WHERE fingerprint = $1 AND status = 'active'`, fp)

	if label = strings.TrimSpace(label); label == "" {
		label = codeLabel
	}
	if label == "" {
		label = "PC"
	}

	var row StationRow
	err = tx.QueryRow(ctx, `
		INSERT INTO b2b_station (org_id, fingerprint, label, status, activated_by, activate_code_id)
		VALUES ($1, $2, $3, 'active', $4, $5)
		RETURNING id, org_id, fingerprint, label, status, activated_at, last_seen_at, activated_by`,
		orgID, fp, label, activatedBy, codeID,
	).Scan(&row.ID, &row.OrgID, &row.Fingerprint, &row.Label, &row.Status,
		&row.ActivatedAt, &row.LastSeenAt, &row.ActivatedBy)
	if err != nil {
		return StationRow{}, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE b2b_station_activate_code SET used_at = now() WHERE id = $1`, codeID); err != nil {
		return StationRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return StationRow{}, err
	}
	row.ActivatedAt = row.ActivatedAt.UTC()
	row.LastSeenAt = row.LastSeenAt.UTC()
	return row, nil
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
		RETURNING id, org_id, fingerprint, label, status, activated_at, last_seen_at, activated_by`,
		stationID, orgID, label,
	).Scan(&row.ID, &row.OrgID, &row.Fingerprint, &row.Label, &row.Status,
		&row.ActivatedAt, &row.LastSeenAt, &row.ActivatedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StationRow{}, ErrNotFound
		}
		return StationRow{}, err
	}
	row.Fingerprint = maskFingerprint(row.Fingerprint)
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
