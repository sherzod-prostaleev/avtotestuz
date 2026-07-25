package admin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// LearnerDirectoryRow is a PII-light row for GET /admin/v1/users.
type LearnerDirectoryRow struct {
	ID           uuid.UUID  `json:"id"`
	PhoneMasked  string     `json:"phone_masked"`
	Name         string     `json:"name"`
	LocalePref   string     `json:"locale_pref"`
	Status       string     `json:"status"`
	VIPActive    bool       `json:"vip_active"`
	Streak       int        `json:"streak"`
	CreatedAt    time.Time  `json:"created_at"`
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
	ReferralCode string     `json:"referral_code,omitempty"`
}

// LearnerDetail is GET /admin/v1/users/{id} (never includes password).
type LearnerDetail struct {
	ID           uuid.UUID  `json:"id"`
	Phone        string     `json:"phone"`
	PhoneMasked  string     `json:"phone_masked"`
	Name         string     `json:"name"`
	Region       string     `json:"region"`
	District     string     `json:"district"`
	LocalePref   string     `json:"locale_pref"`
	ThemePref    string     `json:"theme_pref"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	ReferralCode string     `json:"referral_code,omitempty"`
	ReferredBy   *uuid.UUID `json:"referred_by,omitempty"`
	VIPActive    bool       `json:"vip_active"`
	VIPEndsAt    *time.Time `json:"vip_ends_at,omitempty"`
	Streak       int        `json:"streak"`
	CreatedAt    time.Time  `json:"created_at"`
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
}

// LearnerSessionRow is a refresh_token session for admin security tab.
type LearnerSessionRow struct {
	ID        uuid.UUID  `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	Active    bool       `json:"active"`
}

// ListLearnersResult is a paginated directory page.
type ListLearnersResult struct {
	Items []LearnerDirectoryRow `json:"items"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
	Total int                   `json:"total"`
}

// ListLearners returns profiles matching q (phone/name/uuid/referral), newest first.
func (s Store) ListLearners(ctx context.Context, q string, page, limit int) (ListLearnersResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q = strings.TrimSpace(q)
	offset := (page - 1) * limit

	var total int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM profile p
		WHERE ($1 = '' OR
		  p.phone ILIKE '%' || $1 || '%' OR
		  p.name ILIKE '%' || $1 || '%' OR
		  p.id::text ILIKE $1 || '%' OR
		  COALESCE(p.referral_code, '') ILIKE '%' || $1 || '%')`, q).Scan(&total)
	if err != nil {
		return ListLearnersResult{}, err
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT p.id, p.phone, p.name, p.locale_pref::text, p.status,
		       COALESCE(p.referral_code, ''),
		       p.created_at,
		       EXISTS (
		         SELECT 1 FROM entitlement e
		         WHERE e.profile_id = p.id AND e.ends_at > now()
		       ) AS vip_active,
		       COALESCE(st.current, 0) AS streak,
		       (
		         SELECT MAX(d.last_seen) FROM device d WHERE d.profile_id = p.id
		       ) AS last_seen_at
		FROM profile p
		LEFT JOIN streak st ON st.profile_id = p.id
		WHERE ($1 = '' OR
		  p.phone ILIKE '%' || $1 || '%' OR
		  p.name ILIKE '%' || $1 || '%' OR
		  p.id::text ILIKE $1 || '%' OR
		  COALESCE(p.referral_code, '') ILIKE '%' || $1 || '%')
		ORDER BY p.created_at DESC
		LIMIT $2 OFFSET $3`, q, limit, offset)
	if err != nil {
		return ListLearnersResult{}, err
	}
	defer rows.Close()

	items := make([]LearnerDirectoryRow, 0)
	for rows.Next() {
		var (
			pid                           uuid.UUID
			phone, name, locale, status, ref string
			created                       time.Time
			vip                           bool
			streak                        int
			lastSeen                      *time.Time
		)
		if err := rows.Scan(&pid, &phone, &name, &locale, &status, &ref, &created, &vip, &streak, &lastSeen); err != nil {
			return ListLearnersResult{}, err
		}
		row := LearnerDirectoryRow{
			ID:          pid,
			PhoneMasked: maskPhone(phone),
			Name:        name,
			LocalePref:  locale,
			Status:      mapLearnerStatus(status),
			VIPActive:   vip,
			Streak:      streak,
			CreatedAt:   created.UTC(),
		}
		if ref != "" {
			row.ReferralCode = ref
		}
		if lastSeen != nil {
			t := lastSeen.UTC()
			row.LastSeenAt = &t
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return ListLearnersResult{}, err
	}
	return ListLearnersResult{Items: items, Page: page, Limit: limit, Total: total}, nil
}

// GetLearner returns one profile with VIP/streak/last_seen (no password).
func (s Store) GetLearner(ctx context.Context, id uuid.UUID) (LearnerDetail, error) {
	var d LearnerDetail
	var phone, ref string
	var referredBy *uuid.UUID
	var vipEnds *time.Time
	var lastSeen *time.Time
	var status string
	err := s.Pool.QueryRow(ctx, `
		SELECT p.id, p.phone, p.name, p.region, p.district,
		       p.locale_pref::text, p.theme_pref, p.role, p.status,
		       COALESCE(p.referral_code, ''), p.referred_by, p.created_at,
		       EXISTS (
		         SELECT 1 FROM entitlement e
		         WHERE e.profile_id = p.id AND e.ends_at > now()
		       ),
		       (
		         SELECT e.ends_at FROM entitlement e
		         WHERE e.profile_id = p.id AND e.ends_at > now()
		         ORDER BY e.ends_at DESC LIMIT 1
		       ),
		       COALESCE(st.current, 0),
		       (
		         SELECT MAX(d.last_seen) FROM device d WHERE d.profile_id = p.id
		       )
		FROM profile p
		LEFT JOIN streak st ON st.profile_id = p.id
		WHERE p.id = $1`, id).Scan(
		&d.ID, &phone, &d.Name, &d.Region, &d.District,
		&d.LocalePref, &d.ThemePref, &d.Role, &status,
		&ref, &referredBy, &d.CreatedAt,
		&d.VIPActive, &vipEnds, &d.Streak, &lastSeen,
	)
	if err != nil {
		return LearnerDetail{}, err
	}
	d.Phone = phone
	d.PhoneMasked = maskPhone(phone)
	d.Status = mapLearnerStatus(status)
	d.CreatedAt = d.CreatedAt.UTC()
	if ref != "" {
		d.ReferralCode = ref
	}
	d.ReferredBy = referredBy
	if vipEnds != nil {
		t := vipEnds.UTC()
		d.VIPEndsAt = &t
	}
	if lastSeen != nil {
		t := lastSeen.UTC()
		d.LastSeenAt = &t
	}
	return d, nil
}

// SetLearnerStatus sets profile.status to active|banned (block uses banned).
func (s Store) SetLearnerStatus(ctx context.Context, id uuid.UUID, status string) (before, after string, err error) {
	status = strings.TrimSpace(status)
	if status != "active" && status != "banned" {
		return "", "", fmt.Errorf("invalid status")
	}
	err = s.Pool.QueryRow(ctx, `SELECT status FROM profile WHERE id = $1`, id).Scan(&before)
	if err != nil {
		return "", "", err
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE profile SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return "", "", err
	}
	if tag.RowsAffected() == 0 {
		return "", "", pgx.ErrNoRows
	}
	return before, status, nil
}

// ListLearnerSessions returns refresh_token rows newest-first.
func (s Store) ListLearnerSessions(ctx context.Context, profileID uuid.UUID) ([]LearnerSessionRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, created_at, expires_at, revoked_at
		FROM refresh_token
		WHERE profile_id = $1
		ORDER BY created_at DESC
		LIMIT 100`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]LearnerSessionRow, 0)
	now := time.Now()
	for rows.Next() {
		var row LearnerSessionRow
		var revoked *time.Time
		if err := rows.Scan(&row.ID, &row.CreatedAt, &row.ExpiresAt, &revoked); err != nil {
			return nil, err
		}
		row.CreatedAt = row.CreatedAt.UTC()
		row.ExpiresAt = row.ExpiresAt.UTC()
		if revoked != nil {
			t := revoked.UTC()
			row.RevokedAt = &t
		}
		row.Active = revoked == nil && row.ExpiresAt.After(now)
		out = append(out, row)
	}
	return out, rows.Err()
}

// RevokeLearnerSession revokes one refresh_token if owned by profile.
func (s Store) RevokeLearnerSession(ctx context.Context, profileID, sessionID uuid.UUID) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE refresh_token SET revoked_at = now()
		WHERE id = $1 AND profile_id = $2 AND revoked_at IS NULL`, sessionID, profileID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// RevokeAllLearnerSessions revokes all active refresh tokens for a profile.
func (s Store) RevokeAllLearnerSessions(ctx context.Context, profileID uuid.UUID) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE refresh_token SET revoked_at = now()
		WHERE profile_id = $1 AND revoked_at IS NULL`, profileID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// LearnerExists reports whether profile id is present.
func (s Store) LearnerExists(ctx context.Context, id uuid.UUID) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM profile WHERE id = $1)`, id).Scan(&ok)
	return ok, err
}

func mapLearnerStatus(dbStatus string) string {
	switch dbStatus {
	case "banned":
		return "blocked"
	default:
		return dbStatus
	}
}

func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	n := len(phone)
	if n <= 4 {
		return "***"
	}
	if n <= 8 {
		return phone[:2] + "***" + phone[n-2:]
	}
	return phone[:4] + "***" + phone[n-4:]
}
