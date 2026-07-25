// Package b2b implements the thin M5 teacher/org-owner read portal.
package b2b

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store reads b2b_org* for learner teacher portal.
type Store struct {
	Pool *pgxpool.Pool
}

// OrgSummary is a teacher-visible org row.
type OrgSummary struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	MyRole      string    `json:"my_role"`
	MemberCount int64     `json:"member_count"`
	ActiveSeats int64     `json:"active_seats"`
}

// MemberRow is a PII-light org member.
type MemberRow struct {
	ProfileID   uuid.UUID `json:"profile_id"`
	PhoneMasked string    `json:"phone_masked"`
	Name        string    `json:"name"`
	Role        string    `json:"role"`
}

// LicenseRow is a seat window summary.
type LicenseRow struct {
	ID       uuid.UUID `json:"id"`
	Seats    int       `json:"seats"`
	EndsAt   time.Time `json:"ends_at"`
	Active   bool      `json:"active"`
	Note     string    `json:"note"`
}

// OrgDetail is teacher dashboard payload.
type OrgDetail struct {
	Org      OrgSummary  `json:"org"`
	Members  []MemberRow `json:"members"`
	Licenses []LicenseRow `json:"licenses"`
}

// ListTeacherOrgs returns orgs where the profile is owner or teacher.
func (s Store) ListTeacherOrgs(ctx context.Context, profileID uuid.UUID) ([]OrgSummary, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT o.id, o.name, o.status, m.role,
		       (SELECT COUNT(*) FROM b2b_org_member x WHERE x.org_id = o.id),
		       (SELECT COALESCE(SUM(l.seats), 0) FROM b2b_org_license l
		         WHERE l.org_id = o.id AND l.starts_at <= now() AND l.ends_at > now())
		FROM b2b_org o
		JOIN b2b_org_member m ON m.org_id = o.id AND m.profile_id = $1
		WHERE m.role IN ('owner', 'teacher')
		ORDER BY o.created_at DESC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list teacher orgs: %w", err)
	}
	defer rows.Close()
	out := make([]OrgSummary, 0)
	for rows.Next() {
		var row OrgSummary
		if err := rows.Scan(&row.ID, &row.Name, &row.Status, &row.MyRole, &row.MemberCount, &row.ActiveSeats); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// GetTeacherOrgDetail returns org detail if caller is owner/teacher of that org.
func (s Store) GetTeacherOrgDetail(ctx context.Context, profileID, orgID uuid.UUID) (OrgDetail, error) {
	var out OrgDetail
	err := s.Pool.QueryRow(ctx, `
		SELECT o.id, o.name, o.status, m.role
		FROM b2b_org o
		JOIN b2b_org_member m ON m.org_id = o.id AND m.profile_id = $1
		WHERE o.id = $2 AND m.role IN ('owner', 'teacher')`,
		profileID, orgID,
	).Scan(&out.Org.ID, &out.Org.Name, &out.Org.Status, &out.Org.MyRole)
	if err != nil {
		if err == pgx.ErrNoRows {
			return out, err
		}
		return out, fmt.Errorf("get teacher org: %w", err)
	}

	mrows, err := s.Pool.Query(ctx, `
		SELECT m.profile_id, p.phone, p.name, m.role
		FROM b2b_org_member m
		JOIN profile p ON p.id = m.profile_id
		WHERE m.org_id = $1
		ORDER BY
		  CASE m.role WHEN 'owner' THEN 0 WHEN 'teacher' THEN 1 ELSE 2 END,
		  m.created_at`, orgID)
	if err != nil {
		return out, err
	}
	defer mrows.Close()
	out.Members = make([]MemberRow, 0)
	for mrows.Next() {
		var row MemberRow
		var phone string
		if err := mrows.Scan(&row.ProfileID, &phone, &row.Name, &row.Role); err != nil {
			return out, err
		}
		row.PhoneMasked = maskPhone(phone)
		out.Members = append(out.Members, row)
	}
	if err := mrows.Err(); err != nil {
		return out, err
	}
	out.Org.MemberCount = int64(len(out.Members))

	lrows, err := s.Pool.Query(ctx, `
		SELECT id, seats, ends_at, note,
		       (starts_at <= now() AND ends_at > now()) AS active
		FROM b2b_org_license
		WHERE org_id = $1
		ORDER BY ends_at DESC`, orgID)
	if err != nil {
		return out, err
	}
	defer lrows.Close()
	out.Licenses = make([]LicenseRow, 0)
	var seats int64
	for lrows.Next() {
		var row LicenseRow
		if err := lrows.Scan(&row.ID, &row.Seats, &row.EndsAt, &row.Note, &row.Active); err != nil {
			return out, err
		}
		row.EndsAt = row.EndsAt.UTC()
		if row.Active {
			seats += int64(row.Seats)
		}
		out.Licenses = append(out.Licenses, row)
	}
	out.Org.ActiveSeats = seats
	return out, lrows.Err()
}

func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) < 4 {
		return "***"
	}
	return phone[:len(phone)-4] + "****"
}
