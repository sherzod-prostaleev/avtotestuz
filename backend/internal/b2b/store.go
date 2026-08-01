// Package b2b implements school org teacher/owner portal (M5-02…M5-04).
package b2b

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/auth"
)

var (
	// ErrForbidden is returned when the caller lacks owner/teacher (or owner-only) rights.
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound is a missing org/member/invite.
	ErrNotFound = errors.New("not found")
	// ErrConflict covers last-owner and already-member cases.
	ErrConflict = errors.New("conflict")
	// ErrInvalid is bad input (phone, role, token).
	ErrInvalid = errors.New("invalid")
)

// Store reads/writes b2b_org* for the learner teacher portal.
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
	SeatsUsed   int64     `json:"seats_used"`
}

// MemberRow is a PII-light org member with cheap progress.
type MemberRow struct {
	ProfileID     uuid.UUID `json:"profile_id"`
	PhoneMasked   string    `json:"phone_masked"`
	Name          string    `json:"name"`
	Role          string    `json:"role"`
	SessionsCount int64     `json:"sessions_count"`
	ReadinessPct  int       `json:"readiness_pct"`
	StreakCurrent int       `json:"streak_current"`
	HasB2BVIP     bool      `json:"has_b2b_vip"`
}

// LicenseRow is a seat window summary.
type LicenseRow struct {
	ID     uuid.UUID `json:"id"`
	Seats  int       `json:"seats"`
	EndsAt time.Time `json:"ends_at"`
	Active bool      `json:"active"`
	Note   string    `json:"note"`
}

// OrgDetail is teacher dashboard payload.
type OrgDetail struct {
	Org       OrgSummary   `json:"org"`
	Members   []MemberRow  `json:"members"`
	Licenses  []LicenseRow `json:"licenses"`
	SeatsUsed int64        `json:"seats_used"`
}

// InviteRow is a pending enroll invite.
type InviteRow struct {
	ID          uuid.UUID `json:"id"`
	Token       string    `json:"token,omitempty"`
	PhoneMasked string    `json:"phone_masked"`
	Role        string    `json:"role"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// InviteResult is the outcome of invite-by-phone.
type InviteResult struct {
	Status string     `json:"status"` // enrolled | pending
	Member *MemberRow `json:"member,omitempty"`
	Invite *InviteRow `json:"invite,omitempty"`
}

// OrgStats aggregates for the teacher/admin dashboard.
type OrgStats struct {
	OrgID               uuid.UUID `json:"org_id"`
	MembersTotal        int64     `json:"members_total"`
	Owners              int64     `json:"owners"`
	Teachers            int64     `json:"teachers"`
	Students            int64     `json:"students"`
	ActiveSeats         int64     `json:"active_seats"`
	SeatsUsed           int64     `json:"seats_used"`
	SeatsRemaining      int64     `json:"seats_remaining"`
	ActiveB2BVIP        int64     `json:"active_b2b_vip"`
	AvgReadinessPct     int       `json:"avg_readiness_pct"`
	SessionsFinished7d  int64     `json:"sessions_finished_7d"`
	SessionsFinished30d int64     `json:"sessions_finished_30d"`
	ActiveMembers7d     int64     `json:"active_members_7d"`
	PendingInvites      int64     `json:"pending_invites"`
}

func (s Store) teacherRole(ctx context.Context, profileID, orgID uuid.UUID) (string, error) {
	var role string
	err := s.Pool.QueryRow(ctx, `
		SELECT m.role FROM b2b_org_member m
		JOIN b2b_org o ON o.id = m.org_id
		WHERE m.org_id = $1 AND m.profile_id = $2 AND m.role IN ('owner', 'teacher')`,
		orgID, profileID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return role, nil
}

func (s Store) requireOwner(ctx context.Context, profileID, orgID uuid.UUID) error {
	role, err := s.teacherRole(ctx, profileID, orgID)
	if err != nil {
		return err
	}
	if role != "owner" {
		return ErrForbidden
	}
	return nil
}

// ListTeacherOrgs returns orgs where the profile is owner or teacher.
func (s Store) ListTeacherOrgs(ctx context.Context, profileID uuid.UUID) ([]OrgSummary, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT o.id, o.name, o.status, m.role,
		       (SELECT COUNT(*) FROM b2b_org_member x WHERE x.org_id = o.id),
		       (SELECT COALESCE(SUM(l.seats), 0) FROM b2b_org_license l
		         WHERE l.org_id = o.id AND l.starts_at <= now() AND l.ends_at > now()),
		       (SELECT COUNT(DISTINCT e.profile_id)
		          FROM entitlement e
		          JOIN b2b_org_member bm ON bm.profile_id = e.profile_id AND bm.org_id = o.id
		         WHERE e.source = 'b2b' AND e.starts_at <= now() AND e.ends_at > now()
		           AND e.note LIKE 'b2b_org=' || o.id::text || '%')
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
		if err := rows.Scan(&row.ID, &row.Name, &row.Status, &row.MyRole,
			&row.MemberCount, &row.ActiveSeats, &row.SeatsUsed); err != nil {
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
		if errors.Is(err, pgx.ErrNoRows) {
			return out, ErrNotFound
		}
		return out, fmt.Errorf("get teacher org: %w", err)
	}

	members, err := s.listMembersWithProgress(ctx, orgID)
	if err != nil {
		return out, err
	}
	out.Members = members
	out.Org.MemberCount = int64(len(members))

	licenses, seats, err := s.listLicenses(ctx, orgID)
	if err != nil {
		return out, err
	}
	out.Licenses = licenses
	out.Org.ActiveSeats = seats

	used, err := s.countActiveB2BGrants(ctx, orgID)
	if err != nil {
		return out, err
	}
	out.SeatsUsed = used
	out.Org.SeatsUsed = used
	return out, nil
}

func (s Store) listLicenses(ctx context.Context, orgID uuid.UUID) ([]LicenseRow, int64, error) {
	lrows, err := s.Pool.Query(ctx, `
		SELECT id, seats, ends_at, note,
		       (starts_at <= now() AND ends_at > now()) AS active
		FROM b2b_org_license
		WHERE org_id = $1
		ORDER BY ends_at DESC`, orgID)
	if err != nil {
		return nil, 0, err
	}
	defer lrows.Close()
	out := make([]LicenseRow, 0)
	var seats int64
	for lrows.Next() {
		var row LicenseRow
		if err := lrows.Scan(&row.ID, &row.Seats, &row.EndsAt, &row.Note, &row.Active); err != nil {
			return nil, 0, err
		}
		row.EndsAt = row.EndsAt.UTC()
		if row.Active {
			seats += int64(row.Seats)
		}
		out = append(out, row)
	}
	return out, seats, lrows.Err()
}

func (s Store) countActiveB2BGrants(ctx context.Context, orgID uuid.UUID) (int64, error) {
	var n int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT e.profile_id)
		FROM entitlement e
		JOIN b2b_org_member m ON m.profile_id = e.profile_id AND m.org_id = $1
		WHERE e.source = 'b2b'
		  AND e.starts_at <= now() AND e.ends_at > now()
		  AND e.note LIKE $2`,
		orgID, "b2b_org="+orgID.String()+"%",
	).Scan(&n)
	return n, err
}

func (s Store) listMembersWithProgress(ctx context.Context, orgID uuid.UUID) ([]MemberRow, error) {
	mrows, err := s.Pool.Query(ctx, `
		WITH qcounts AS (
		  SELECT category_id, COUNT(*)::bigint AS n
		  FROM question WHERE validation_status = 'valid'
		  GROUP BY category_id
		),
		studied AS (
		  SELECT qm.profile_id, q.category_id, COUNT(*)::bigint AS n
		  FROM question_memory qm
		  JOIN question q ON q.id = qm.question_id AND q.validation_status = 'valid'
		  GROUP BY qm.profile_id, q.category_id
		),
		member_profiles AS (
		  SELECT profile_id FROM b2b_org_member WHERE org_id = $1
		),
		readiness AS (
		  SELECT mp.profile_id,
		         COALESCE(ROUND(100.0 * SUM(
		           (LEAST(COALESCE(st.n, 0), qc.n)::float8 / NULLIF(qc.n, 0)::float8)
		           * CASE WHEN COALESCE(cm.seen, 0) > 0
		                  THEN cm.correct::float8 / cm.seen::float8
		                  ELSE 0 END
		           * qc.n::float8
		         ) / NULLIF(SUM(qc.n), 0)::float8), 0)::int AS pct
		  FROM member_profiles mp
		  CROSS JOIN qcounts qc
		  LEFT JOIN category_mastery cm
		    ON cm.profile_id = mp.profile_id AND cm.category_id = qc.category_id
		  LEFT JOIN studied st
		    ON st.profile_id = mp.profile_id AND st.category_id = qc.category_id
		  GROUP BY mp.profile_id
		)
		SELECT m.profile_id, p.phone, p.name, m.role,
		       (SELECT COUNT(*) FROM exam_session es WHERE es.profile_id = m.profile_id),
		       COALESCE(r.pct, 0),
		       COALESCE(st.current, 0),
		       EXISTS(
		         SELECT 1 FROM entitlement e
		         WHERE e.profile_id = m.profile_id AND e.source = 'b2b'
		           AND e.starts_at <= now() AND e.ends_at > now()
		           AND e.note LIKE $2
		       )
		FROM b2b_org_member m
		JOIN profile p ON p.id = m.profile_id
		LEFT JOIN readiness r ON r.profile_id = m.profile_id
		LEFT JOIN streak st ON st.profile_id = m.profile_id
		WHERE m.org_id = $1
		ORDER BY
		  CASE m.role WHEN 'owner' THEN 0 WHEN 'teacher' THEN 1 ELSE 2 END,
		  m.created_at`, orgID, "b2b_org="+orgID.String()+"%")
	if err != nil {
		return nil, err
	}
	defer mrows.Close()
	out := make([]MemberRow, 0)
	for mrows.Next() {
		var row MemberRow
		var phone string
		if err := mrows.Scan(&row.ProfileID, &phone, &row.Name, &row.Role,
			&row.SessionsCount, &row.ReadinessPct, &row.StreakCurrent, &row.HasB2BVIP); err != nil {
			return nil, err
		}
		row.PhoneMasked = maskPhone(phone)
		out = append(out, row)
	}
	return out, mrows.Err()
}

func (s Store) getMemberRow(ctx context.Context, orgID, profileID uuid.UUID) (MemberRow, error) {
	members, err := s.listMembersWithProgress(ctx, orgID)
	if err != nil {
		return MemberRow{}, err
	}
	for _, m := range members {
		if m.ProfileID == profileID {
			return m, nil
		}
	}
	return MemberRow{}, ErrNotFound
}

func normalizeRole(role string) (string, error) {
	role = strings.TrimSpace(role)
	if role == "" {
		role = "student"
	}
	switch role {
	case "owner", "teacher", "student":
		return role, nil
	default:
		return "", ErrInvalid
	}
}

func newInviteToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func maskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) < 4 {
		return "***"
	}
	return phone[:len(phone)-4] + "****"
}

// InviteByPhone enrolls an existing profile or creates a pending invite (teacher/owner).
func (s Store) InviteByPhone(ctx context.Context, actorID, orgID uuid.UUID, rawPhone, role string) (InviteResult, error) {
	if _, err := s.teacherRole(ctx, actorID, orgID); err != nil {
		return InviteResult{}, err
	}
	role, err := normalizeRole(role)
	if err != nil {
		return InviteResult{}, err
	}
	if role == "owner" {
		if err := s.requireOwner(ctx, actorID, orgID); err != nil {
			return InviteResult{}, err
		}
	}
	return s.EnrollOrInvite(ctx, orgID, rawPhone, role, actorID)
}

// EnrollOrInvite enrolls by phone if profile exists, else creates a pending invite.
// createdBy may be uuid.Nil (admin invite before any owner); otherwise FK profile id.
func (s Store) EnrollOrInvite(ctx context.Context, orgID uuid.UUID, rawPhone, role string, createdBy uuid.UUID) (InviteResult, error) {
	role, err := normalizeRole(role)
	if err != nil {
		return InviteResult{}, err
	}
	phone, err := auth.NormalizePhone(rawPhone)
	if err != nil {
		return InviteResult{}, fmt.Errorf("%w: phone", ErrInvalid)
	}

	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM b2b_org WHERE id=$1)`, orgID).Scan(&exists); err != nil {
		return InviteResult{}, err
	}
	if !exists {
		return InviteResult{}, ErrNotFound
	}

	var profileID uuid.UUID
	err = s.Pool.QueryRow(ctx, `SELECT id FROM profile WHERE phone = $1`, phone).Scan(&profileID)
	if err == nil {
		if _, err := s.Pool.Exec(ctx, `
			INSERT INTO b2b_org_member (org_id, profile_id, role)
			VALUES ($1, $2, $3)
			ON CONFLICT (org_id, profile_id) DO UPDATE SET role = EXCLUDED.role`,
			orgID, profileID, role); err != nil {
			return InviteResult{}, err
		}
		member, err := s.getMemberRow(ctx, orgID, profileID)
		if err != nil {
			return InviteResult{}, err
		}
		return InviteResult{Status: "enrolled", Member: &member}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return InviteResult{}, err
	}

	token, err := newInviteToken()
	if err != nil {
		return InviteResult{}, err
	}
	expires := time.Now().UTC().Add(14 * 24 * time.Hour)
	var inv InviteRow
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO b2b_invite (token, org_id, phone, role, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, '00000000-0000-0000-0000-000000000000'::uuid))
		RETURNING id, token, role, expires_at, created_at`,
		token, orgID, phone, role, expires, createdBy,
	).Scan(&inv.ID, &inv.Token, &inv.Role, &inv.ExpiresAt, &inv.CreatedAt)
	if err != nil {
		return InviteResult{}, err
	}
	inv.ExpiresAt = inv.ExpiresAt.UTC()
	inv.CreatedAt = inv.CreatedAt.UTC()
	inv.PhoneMasked = maskPhone(phone)
	return InviteResult{Status: "pending", Invite: &inv}, nil
}

// FirstOwnerProfile returns an owner profile id for admin attribution (invite created_by).
func (s Store) FirstOwnerProfile(ctx context.Context, orgID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.Pool.QueryRow(ctx, `
		SELECT profile_id FROM b2b_org_member
		WHERE org_id = $1 AND role = 'owner'
		ORDER BY created_at ASC LIMIT 1`, orgID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

// AdminRemoveMember removes a member without teacher ACL (admin permissioned caller).
func (s Store) AdminRemoveMember(ctx context.Context, orgID, targetID uuid.UUID) error {
	var targetRole string
	err := s.Pool.QueryRow(ctx, `
		SELECT role FROM b2b_org_member WHERE org_id = $1 AND profile_id = $2`,
		orgID, targetID).Scan(&targetRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if targetRole == "owner" {
		var owners int64
		if err := s.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM b2b_org_member WHERE org_id = $1 AND role = 'owner'`, orgID).Scan(&owners); err != nil {
			return err
		}
		if owners <= 1 {
			return fmt.Errorf("%w: last owner", ErrConflict)
		}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `
		DELETE FROM b2b_org_member WHERE org_id = $1 AND profile_id = $2`, orgID, targetID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := clampB2BEntitlementsTx(ctx, tx, orgID, targetID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AdminChangeRole updates role without teacher ACL.
func (s Store) AdminChangeRole(ctx context.Context, orgID, targetID uuid.UUID, newRole string) (MemberRow, error) {
	newRole, err := normalizeRole(newRole)
	if err != nil {
		return MemberRow{}, err
	}
	var current string
	err = s.Pool.QueryRow(ctx, `
		SELECT role FROM b2b_org_member WHERE org_id = $1 AND profile_id = $2`,
		orgID, targetID).Scan(&current)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MemberRow{}, ErrNotFound
		}
		return MemberRow{}, err
	}
	if current == "owner" && newRole != "owner" {
		var owners int64
		if err := s.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM b2b_org_member WHERE org_id = $1 AND role = 'owner'`, orgID).Scan(&owners); err != nil {
			return MemberRow{}, err
		}
		if owners <= 1 {
			return MemberRow{}, fmt.Errorf("%w: last owner", ErrConflict)
		}
	}
	if _, err := s.Pool.Exec(ctx, `
		UPDATE b2b_org_member SET role = $3 WHERE org_id = $1 AND profile_id = $2`,
		orgID, targetID, newRole); err != nil {
		return MemberRow{}, err
	}
	return s.getMemberRow(ctx, orgID, targetID)
}

// ListPendingInvites returns open invites for an org (teacher/owner).
func (s Store) ListPendingInvites(ctx context.Context, actorID, orgID uuid.UUID) ([]InviteRow, error) {
	if _, err := s.teacherRole(ctx, actorID, orgID); err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, token, phone, role, expires_at, created_at
		FROM b2b_invite
		WHERE org_id = $1 AND accepted_at IS NULL AND expires_at > now()
		ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]InviteRow, 0)
	for rows.Next() {
		var row InviteRow
		var phone string
		if err := rows.Scan(&row.ID, &row.Token, &phone, &row.Role, &row.ExpiresAt, &row.CreatedAt); err != nil {
			return nil, err
		}
		row.PhoneMasked = maskPhone(phone)
		row.ExpiresAt = row.ExpiresAt.UTC()
		row.CreatedAt = row.CreatedAt.UTC()
		out = append(out, row)
	}
	return out, rows.Err()
}

// AcceptInvite joins the caller's profile into the org using an invite token.
func (s Store) AcceptInvite(ctx context.Context, profileID uuid.UUID, token string) (MemberRow, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return MemberRow{}, ErrInvalid
	}
	var phone string
	err := s.Pool.QueryRow(ctx, `SELECT phone FROM profile WHERE id = $1`, profileID).Scan(&phone)
	if err != nil {
		return MemberRow{}, err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return MemberRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var inviteID, orgID uuid.UUID
	var role, invitePhone string
	err = tx.QueryRow(ctx, `
		SELECT id, org_id, role, phone FROM b2b_invite
		WHERE token = $1 AND accepted_at IS NULL AND expires_at > now()
		FOR UPDATE`, token).Scan(&inviteID, &orgID, &role, &invitePhone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MemberRow{}, ErrNotFound
		}
		return MemberRow{}, err
	}
	if invitePhone != phone {
		return MemberRow{}, ErrForbidden
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO b2b_org_member (org_id, profile_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (org_id, profile_id) DO UPDATE SET role = EXCLUDED.role`,
		orgID, profileID, role); err != nil {
		return MemberRow{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE b2b_invite SET accepted_at = now(), accepted_by = $2 WHERE id = $1`,
		inviteID, profileID); err != nil {
		return MemberRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MemberRow{}, err
	}
	return s.getMemberRow(ctx, orgID, profileID)
}

// PendingInviteView includes org name for the student inbox.
type PendingInviteView struct {
	ID          uuid.UUID `json:"id"`
	Token       string    `json:"token"`
	OrgID       uuid.UUID `json:"org_id"`
	OrgName     string    `json:"org_name"`
	Role        string    `json:"role"`
	ExpiresAt   time.Time `json:"expires_at"`
	PhoneMasked string    `json:"phone_masked"`
}

// ListMyPendingInviteViews is the student-facing invite list.
func (s Store) ListMyPendingInviteViews(ctx context.Context, profileID uuid.UUID) ([]PendingInviteView, error) {
	var phone string
	if err := s.Pool.QueryRow(ctx, `SELECT phone FROM profile WHERE id = $1`, profileID).Scan(&phone); err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT i.id, i.token, i.org_id, o.name, i.role, i.expires_at, i.phone
		FROM b2b_invite i
		JOIN b2b_org o ON o.id = i.org_id
		WHERE i.phone = $1 AND i.accepted_at IS NULL AND i.expires_at > now()
		ORDER BY i.created_at DESC`, phone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PendingInviteView, 0)
	for rows.Next() {
		var row PendingInviteView
		var p string
		if err := rows.Scan(&row.ID, &row.Token, &row.OrgID, &row.OrgName, &row.Role, &row.ExpiresAt, &p); err != nil {
			return nil, err
		}
		row.PhoneMasked = maskPhone(p)
		row.ExpiresAt = row.ExpiresAt.UTC()
		out = append(out, row)
	}
	return out, rows.Err()
}

// RemoveMember removes a member (not the last owner) and clamps their b2b entitlement.
func (s Store) RemoveMember(ctx context.Context, actorID, orgID, targetID uuid.UUID) error {
	role, err := s.teacherRole(ctx, actorID, orgID)
	if err != nil {
		return err
	}
	var targetRole string
	err = s.Pool.QueryRow(ctx, `
		SELECT role FROM b2b_org_member WHERE org_id = $1 AND profile_id = $2`,
		orgID, targetID).Scan(&targetRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if targetRole == "owner" || targetRole == "teacher" {
		if role != "owner" {
			return ErrForbidden
		}
	}
	if targetRole == "owner" {
		var owners int64
		if err := s.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM b2b_org_member WHERE org_id = $1 AND role = 'owner'`, orgID).Scan(&owners); err != nil {
			return err
		}
		if owners <= 1 {
			return fmt.Errorf("%w: last owner", ErrConflict)
		}
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		DELETE FROM b2b_org_member WHERE org_id = $1 AND profile_id = $2`, orgID, targetID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := clampB2BEntitlementsTx(ctx, tx, orgID, targetID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ChangeRole updates a member role (owner only). Cannot demote last owner.
func (s Store) ChangeRole(ctx context.Context, actorID, orgID, targetID uuid.UUID, newRole string) (MemberRow, error) {
	if err := s.requireOwner(ctx, actorID, orgID); err != nil {
		return MemberRow{}, err
	}
	newRole, err := normalizeRole(newRole)
	if err != nil {
		return MemberRow{}, err
	}
	var current string
	err = s.Pool.QueryRow(ctx, `
		SELECT role FROM b2b_org_member WHERE org_id = $1 AND profile_id = $2`,
		orgID, targetID).Scan(&current)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MemberRow{}, ErrNotFound
		}
		return MemberRow{}, err
	}
	if current == "owner" && newRole != "owner" {
		var owners int64
		if err := s.Pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM b2b_org_member WHERE org_id = $1 AND role = 'owner'`, orgID).Scan(&owners); err != nil {
			return MemberRow{}, err
		}
		if owners <= 1 {
			return MemberRow{}, fmt.Errorf("%w: last owner", ErrConflict)
		}
	}
	if _, err := s.Pool.Exec(ctx, `
		UPDATE b2b_org_member SET role = $3 WHERE org_id = $1 AND profile_id = $2`,
		orgID, targetID, newRole); err != nil {
		return MemberRow{}, err
	}
	return s.getMemberRow(ctx, orgID, targetID)
}

func clampB2BEntitlementsTx(ctx context.Context, tx pgx.Tx, orgID, profileID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE entitlement
		SET ends_at = CASE
		      WHEN starts_at >= now() THEN starts_at + interval '1 second'
		      ELSE now()
		    END,
		    note = note || ' | b2b_member_removed'
		WHERE profile_id = $1
		  AND source = 'b2b'
		  AND ends_at > now()
		  AND note LIKE $2`,
		profileID, "b2b_org="+orgID.String()+"%")
	return err
}

// OrgStats computes aggregates for teacher/admin.
func (s Store) OrgStats(ctx context.Context, orgID uuid.UUID) (OrgStats, error) {
	var out OrgStats
	out.OrgID = orgID
	err := s.Pool.QueryRow(ctx, `
		SELECT
		  COUNT(*),
		  COUNT(*) FILTER (WHERE role = 'owner'),
		  COUNT(*) FILTER (WHERE role = 'teacher'),
		  COUNT(*) FILTER (WHERE role = 'student')
		FROM b2b_org_member WHERE org_id = $1`, orgID).Scan(
		&out.MembersTotal, &out.Owners, &out.Teachers, &out.Students)
	if err != nil {
		return out, err
	}
	if err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(seats), 0) FROM b2b_org_license
		WHERE org_id = $1 AND starts_at <= now() AND ends_at > now()`, orgID).Scan(&out.ActiveSeats); err != nil {
		return out, err
	}
	used, err := s.countActiveB2BGrants(ctx, orgID)
	if err != nil {
		return out, err
	}
	out.SeatsUsed = used
	out.ActiveB2BVIP = used
	out.SeatsRemaining = out.ActiveSeats - used
	if out.SeatsRemaining < 0 {
		out.SeatsRemaining = 0
	}

	members, err := s.listMembersWithProgress(ctx, orgID)
	if err != nil {
		return out, err
	}
	if len(members) > 0 {
		sum := 0
		for _, m := range members {
			sum += m.ReadinessPct
		}
		out.AvgReadinessPct = sum / len(members)
	}

	if err := s.Pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE es.finished_at >= now() - interval '7 days'
		                     AND es.status IN ('passed','failed')),
		  COUNT(*) FILTER (WHERE es.finished_at >= now() - interval '30 days'
		                     AND es.status IN ('passed','failed')),
		  COUNT(DISTINCT es.profile_id) FILTER (
		    WHERE es.started_at >= now() - interval '7 days')
		FROM exam_session es
		JOIN b2b_org_member m ON m.profile_id = es.profile_id AND m.org_id = $1`,
		orgID).Scan(&out.SessionsFinished7d, &out.SessionsFinished30d, &out.ActiveMembers7d); err != nil {
		return out, err
	}
	if err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM b2b_invite
		WHERE org_id = $1 AND accepted_at IS NULL AND expires_at > now()`, orgID).Scan(&out.PendingInvites); err != nil {
		return out, err
	}
	return out, nil
}

// ExportMembersCSV builds CSV bytes for org members + progress.
func (s Store) ExportMembersCSV(ctx context.Context, orgID uuid.UUID) ([]byte, error) {
	members, err := s.listMembersWithProgress(ctx, orgID)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if err := w.Write([]string{"profile_id", "phone_masked", "name", "role", "sessions_count", "readiness_pct", "streak_current", "has_b2b_vip"}); err != nil {
		return nil, err
	}
	for _, m := range members {
		vip := "0"
		if m.HasB2BVIP {
			vip = "1"
		}
		record := []string{
			spreadsheetSafe(m.ProfileID.String()), spreadsheetSafe(m.PhoneMasked),
			spreadsheetSafe(m.Name), spreadsheetSafe(m.Role),
			fmt.Sprint(m.SessionsCount), fmt.Sprint(m.ReadinessPct),
			fmt.Sprint(m.StreakCurrent), vip,
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func spreadsheetSafe(value string) string {
	if value != "" && strings.ContainsRune("=+-@\t\r", rune(value[0])) {
		return "'" + value
	}
	return value
}
