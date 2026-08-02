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

// PartnerPromo is a B2C promo tied to a school partner (not resale).
type PartnerPromo struct {
	ID            uuid.UUID  `json:"id"`
	Code          string     `json:"code"`
	Kind          string     `json:"kind"`
	Value         int        `json:"value"`
	PartnerOrgID  uuid.UUID  `json:"partner_org_id"`
	Active        bool       `json:"active"`
	ValidTo       *time.Time `json:"valid_to,omitempty"`
}

// CreatePartnerPromo creates a percent/fixed/days promo linked to a school org.
func (s Store) CreatePartnerPromo(ctx context.Context, orgID uuid.UUID, code, kind string, value int, validDays int) (PartnerPromo, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" || value <= 0 {
		return PartnerPromo{}, fmt.Errorf("%w: code/value", ErrInvalid)
	}
	switch kind {
	case "percent", "fixed", "days":
	default:
		return PartnerPromo{}, fmt.Errorf("%w: kind", ErrInvalid)
	}
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM b2b_org WHERE id=$1 AND status='active')`, orgID).Scan(&exists); err != nil {
		return PartnerPromo{}, err
	}
	if !exists {
		return PartnerPromo{}, ErrNotFound
	}
	var validTo *time.Time
	if validDays > 0 {
		t := time.Now().UTC().Add(time.Duration(validDays) * 24 * time.Hour)
		validTo = &t
	}
	var row PartnerPromo
	var vt pgtype.Timestamptz
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO promo_code (code, kind, value, active, valid_to, partner_org_id, per_user_limit)
		VALUES ($1, $2, $3, true, $4, $5, 1)
		RETURNING id, code, kind, value, partner_org_id, active, valid_to`,
		code, kind, value, validTo, orgID,
	).Scan(&row.ID, &row.Code, &row.Kind, &row.Value, &row.PartnerOrgID, &row.Active, &vt)
	if err != nil {
		if strings.Contains(err.Error(), "promo_code_code_key") || strings.Contains(err.Error(), "duplicate") {
			return PartnerPromo{}, fmt.Errorf("%w: code exists", ErrConflict)
		}
		return PartnerPromo{}, err
	}
	if vt.Valid {
		t := vt.Time.UTC()
		row.ValidTo = &t
	}
	return row, nil
}

// ListPartnerPromos lists promos for an org.
func (s Store) ListPartnerPromos(ctx context.Context, orgID uuid.UUID) ([]PartnerPromo, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, code, kind, value, partner_org_id, active, valid_to
		FROM promo_code WHERE partner_org_id = $1
		ORDER BY code`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PartnerPromo, 0)
	for rows.Next() {
		var row PartnerPromo
		var vt pgtype.Timestamptz
		if err := rows.Scan(&row.ID, &row.Code, &row.Kind, &row.Value, &row.PartnerOrgID, &row.Active, &vt); err != nil {
			return nil, err
		}
		if vt.Valid {
			t := vt.Time.UTC()
			row.ValidTo = &t
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// EnsureOrgExists is a tiny helper for admin handlers.
func (s Store) EnsureOrgExists(ctx context.Context, orgID uuid.UUID) error {
	var ok bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM b2b_org WHERE id=$1)`, orgID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// PartnerOrgForPromo returns partner org id for a promo code, if any.
func (s Store) PartnerOrgForPromo(ctx context.Context, promoID uuid.UUID) (uuid.UUID, bool, error) {
	var orgID uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		SELECT partner_org_id FROM promo_code
		WHERE id = $1 AND partner_org_id IS NOT NULL`, promoID).Scan(&orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, err
	}
	return orgID, true, nil
}
