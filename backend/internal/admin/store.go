// Package admin implements M3 Super Admin staff auth, RBAC, and /admin/v1 APIs.
package admin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"avtotest.uz/backend/internal/auth"
)

const (
	accessTTL  = 15 * time.Minute
	refreshTTL = 30 * 24 * time.Hour
	tokenTyp   = "admin"
)

// Store is the admin persistence layer (raw SQL until sqlc coverage expands).
type Store struct {
	Pool *pgxpool.Pool
}

// User is a staff account row.
type User struct {
	ID            uuid.UUID
	Email         string
	DisplayName   string
	PasswordHash  string
	Status        string
	TotpSecretEnc string // empty when TOTP not enrolled
}

func (u User) TOTPEnabled() bool {
	return strings.TrimSpace(u.TotpSecretEnc) != ""
}

// SessionRow is an admin_session.
type SessionRow struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
}

func (s Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var u User
	var totpEnc *string
	err := s.Pool.QueryRow(ctx, `
		SELECT id, email, display_name, password_hash, status, totp_secret_enc
		FROM admin_user WHERE email = $1`, email).Scan(
		&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Status, &totpEnc,
	)
	if err != nil {
		return User{}, err
	}
	if totpEnc != nil {
		u.TotpSecretEnc = *totpEnc
	}
	return u, nil
}

func (s Store) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	var totpEnc *string
	err := s.Pool.QueryRow(ctx, `
		SELECT id, email, display_name, password_hash, status, totp_secret_enc
		FROM admin_user WHERE id = $1`, id).Scan(
		&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Status, &totpEnc,
	)
	if err != nil {
		return User{}, err
	}
	if totpEnc != nil {
		u.TotpSecretEnc = *totpEnc
	}
	return u, nil
}

func (s Store) SetUserTOTPSecret(ctx context.Context, id uuid.UUID, enc string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE admin_user SET totp_secret_enc = NULLIF($2, ''), updated_at = now()
		WHERE id = $1`, id, enc)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s Store) ListPermissions(ctx context.Context, adminUserID uuid.UUID) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT p.code
		FROM admin_permission p
		JOIN admin_role_permission rp ON rp.permission_id = p.id
		JOIN admin_user_role ur ON ur.role_id = rp.role_id
		WHERE ur.admin_user_id = $1
		ORDER BY p.code`, adminUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}

func (s Store) ListRoles(ctx context.Context, adminUserID uuid.UUID) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT r.code
		FROM admin_role r
		JOIN admin_user_role ur ON ur.role_id = r.id
		WHERE ur.admin_user_id = $1
		ORDER BY r.code`, adminUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}

func (s Store) CreateSession(ctx context.Context, userID uuid.UUID, refreshHash, ua string, ip *net.IP, expires time.Time) (uuid.UUID, error) {
	id := uuid.New()
	var ipArg any
	if ip != nil {
		ipArg = *ip
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO admin_session (id, admin_user_id, refresh_hash, ip, ua, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		id, userID, refreshHash, ipArg, ua, expires,
	)
	return id, err
}

func (s Store) FindSessionByRefreshHash(ctx context.Context, hash string) (SessionRow, error) {
	var row SessionRow
	err := s.Pool.QueryRow(ctx, `
		SELECT id, admin_user_id, expires_at
		FROM admin_session
		WHERE refresh_hash = $1 AND revoked_at IS NULL`, hash).Scan(&row.ID, &row.UserID, &row.ExpiresAt)
	return row, err
}

func (s Store) RotateSession(ctx context.Context, sessionID uuid.UUID, oldHash, newHash string, expires time.Time) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE admin_session SET refresh_hash = $2, expires_at = $3
		WHERE id = $1 AND refresh_hash = $4 AND revoked_at IS NULL`, sessionID, newHash, expires, oldHash)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s Store) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE admin_session SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, sessionID)
	return err
}

func (s Store) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE admin_session SET revoked_at = now()
		WHERE admin_user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// EnsureSuperadmin upserts a local seed superadmin (idempotent by email).
func (s Store) EnsureSuperadmin(ctx context.Context, email, password, displayName string) (uuid.UUID, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return uuid.Nil, fmt.Errorf("email and password required")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return uuid.Nil, err
	}
	if displayName == "" {
		displayName = "Superadmin"
	}

	var id uuid.UUID
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO admin_user (email, display_name, password_hash, status)
		VALUES ($1, $2, $3, 'active')
		ON CONFLICT (email) DO UPDATE
		  SET password_hash = EXCLUDED.password_hash,
		      display_name = EXCLUDED.display_name,
		      status = 'active',
		      updated_at = now()
		RETURNING id`, email, displayName, hash).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}

	_, err = s.Pool.Exec(ctx, `
		INSERT INTO admin_user_role (admin_user_id, role_id)
		SELECT $1, r.id FROM admin_role r WHERE r.code = 'superadmin'
		ON CONFLICT DO NOTHING`, id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// WriteAudit appends an immutable admin_audit_log row.
func (s Store) WriteAudit(ctx context.Context, adminUserID *uuid.UUID, action, entityType, entityID string, before, after any, ip *net.IP, ua, requestID string) error {
	var ipArg any
	if ip != nil {
		ipArg = *ip
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO admin_audit_log
		  (admin_user_id, action, entity_type, entity_id, before_json, after_json, ip, ua, request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		adminUserID, action, entityType, nullIfEmpty(entityID), before, after, ipArg, ua, requestID,
	)
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// IsNoRows reports pgx.ErrNoRows, including when it arrives wrapped — a ==
// comparison silently answered false for any caller that added context with
// %w, turning "not found" into a 500.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
