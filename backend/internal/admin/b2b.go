package admin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"avtotest.uz/backend/internal/b2b"
)

// B2BOrgRow is a list/detail org.
type B2BOrgRow struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Seats     int64     `json:"active_seats,omitempty"`
}

// B2BLicenseRow is a seat license window.
type B2BLicenseRow struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"org_id"`
	Seats     int       `json:"seats"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Note      string    `json:"note"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Active    bool      `json:"active"`
}

// B2BOrgDetail bundles org + licenses + stations.
type B2BOrgDetail struct {
	Org       B2BOrgRow        `json:"org"`
	Licenses  []B2BLicenseRow  `json:"licenses"`
	Stations  []b2b.StationRow `json:"stations"`
	SeatsUsed int64            `json:"seats_used"`
}

func (s Store) ListB2BOrgs(ctx context.Context) ([]B2BOrgRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT o.id, o.name, o.status, o.created_at,
		       (SELECT COALESCE(SUM(l.seats), 0) FROM b2b_org_license l
		         WHERE l.org_id = o.id AND l.starts_at <= now() AND l.ends_at > now())
		FROM b2b_org o
		ORDER BY o.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list b2b orgs: %w", err)
	}
	defer rows.Close()
	out := make([]B2BOrgRow, 0)
	for rows.Next() {
		var row B2BOrgRow
		if err := rows.Scan(&row.ID, &row.Name, &row.Status, &row.CreatedAt, &row.Seats); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s Store) CreateB2BOrg(ctx context.Context, name string) (B2BOrgRow, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return B2BOrgRow{}, fmt.Errorf("name required")
	}
	var row B2BOrgRow
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO b2b_org (name) VALUES ($1)
		RETURNING id, name, status, created_at`, name).Scan(&row.ID, &row.Name, &row.Status, &row.CreatedAt)
	return row, err
}

func (s Store) GetB2BOrgDetail(ctx context.Context, orgID uuid.UUID) (B2BOrgDetail, error) {
	var out B2BOrgDetail
	err := s.Pool.QueryRow(ctx, `
		SELECT id, name, status, created_at FROM b2b_org WHERE id = $1`, orgID).Scan(
		&out.Org.ID, &out.Org.Name, &out.Org.Status, &out.Org.CreatedAt)
	if err != nil {
		return out, err
	}
	lrows, err := s.Pool.Query(ctx, `
		SELECT id, org_id, seats, starts_at, ends_at, note, created_by, created_at,
		       (starts_at <= now() AND ends_at > now()) AS active
		FROM b2b_org_license WHERE org_id = $1
		ORDER BY created_at DESC`, orgID)
	if err != nil {
		return out, err
	}
	defer lrows.Close()
	out.Licenses = make([]B2BLicenseRow, 0)
	for lrows.Next() {
		var row B2BLicenseRow
		if err := lrows.Scan(&row.ID, &row.OrgID, &row.Seats, &row.StartsAt, &row.EndsAt,
			&row.Note, &row.CreatedBy, &row.CreatedAt, &row.Active); err != nil {
			return out, err
		}
		out.Licenses = append(out.Licenses, row)
	}
	if err := lrows.Err(); err != nil {
		return out, err
	}
	for _, l := range out.Licenses {
		if l.Active {
			out.Org.Seats += int64(l.Seats)
		}
	}
	bs := b2b.Store{Pool: s.Pool}
	stations, err := bs.ListStations(ctx, orgID)
	if err != nil {
		return out, err
	}
	out.Stations = stations
	used, err := bs.CountActiveStations(ctx, orgID)
	if err != nil {
		return out, err
	}
	out.SeatsUsed = used
	return out, nil
}

func (s Store) CreateB2BLicense(ctx context.Context, orgID uuid.UUID, seats, days int, note, createdBy string) (B2BLicenseRow, error) {
	if seats <= 0 || days <= 0 {
		return B2BLicenseRow{}, fmt.Errorf("seats and days must be positive")
	}
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM b2b_org WHERE id=$1)`, orgID).Scan(&exists); err != nil {
		return B2BLicenseRow{}, err
	}
	if !exists {
		return B2BLicenseRow{}, pgx.ErrNoRows
	}
	start := time.Now().UTC()
	end := start.Add(time.Duration(days) * 24 * time.Hour)
	var row B2BLicenseRow
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO b2b_org_license (org_id, seats, starts_at, ends_at, note, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, org_id, seats, starts_at, ends_at, note, created_by, created_at`,
		orgID, seats, start, end, note, createdBy,
	).Scan(&row.ID, &row.OrgID, &row.Seats, &row.StartsAt, &row.EndsAt, &row.Note, &row.CreatedBy, &row.CreatedAt)
	row.Active = true
	return row, err
}

// HardDeleteB2BOrg permanently removes an organization and every station,
// enroll code, license and partner promotion tied to it. Profiles are
// deliberately preserved.
func (s Store) HardDeleteB2BOrg(ctx context.Context, orgID uuid.UUID, confirm string, audit MutationAudit) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var name, status string
	if err := tx.QueryRow(ctx, `
		SELECT name, status FROM b2b_org WHERE id = $1 FOR UPDATE`, orgID,
	).Scan(&name, &status); err != nil {
		return err
	}
	if strings.TrimSpace(confirm) != name {
		return ErrDeleteConfirmation
	}
	var stations, licenses int64
	if err := tx.QueryRow(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM b2b_station WHERE org_id = $1),
		  (SELECT COUNT(*) FROM b2b_org_license WHERE org_id = $1)`, orgID,
	).Scan(&stations, &licenses); err != nil {
		return err
	}
	if err := writeAuditTx(ctx, tx, audit, "b2b.orgs.hard_delete", "b2b_org", orgID.String(),
		map[string]any{
			"id": orgID.String(), "name": name, "status": status,
			"stations": stations, "licenses": licenses,
		},
		map[string]any{"deleted": true},
	); err != nil {
		return fmt.Errorf("write org hard-delete audit: %w", err)
	}

	// Historical VIP grants are profile entitlements and therefore do not
	// cascade from b2b_org. Revoke only grants carrying this org's structured
	// note; nothing mints new ones this way anymore, but rows created before
	// this migration can still exist.
	if _, err := tx.Exec(ctx, `
		DELETE FROM entitlement
		WHERE source = 'b2b' AND note LIKE $1`, "b2b_org="+orgID.String()+";%"); err != nil {
		return err
	}
	// Partner promo usage/payments are retained, but the org-owned promo itself
	// and its redemption rows are removed. Detaching payment.promo_code_id keeps
	// immutable customer payment history intact.
	if _, err := tx.Exec(ctx, `
		UPDATE payment SET promo_code_id = NULL
		WHERE promo_code_id IN (SELECT id FROM promo_code WHERE partner_org_id = $1)`, orgID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM promo_redemption
		WHERE promo_code_id IN (SELECT id FROM promo_code WHERE partner_org_id = $1)`, orgID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM promo_code WHERE partner_org_id = $1`, orgID); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `DELETE FROM b2b_org WHERE id = $1`, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return tx.Commit(ctx)
}
