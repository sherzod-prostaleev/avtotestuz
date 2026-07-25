package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AdminAuditRow is one admin_audit_log entry for the Security → Audit UI.
type AdminAuditRow struct {
	ID          uuid.UUID       `json:"id"`
	AdminUserID *uuid.UUID      `json:"admin_user_id,omitempty"`
	AdminEmail  string          `json:"admin_email,omitempty"`
	Action      string          `json:"action"`
	EntityType  string          `json:"entity_type"`
	EntityID    *string         `json:"entity_id,omitempty"`
	BeforeJSON  json.RawMessage `json:"before_json,omitempty"`
	AfterJSON   json.RawMessage `json:"after_json,omitempty"`
	IP          *string         `json:"ip,omitempty"`
	UA          string          `json:"ua,omitempty"`
	RequestID   string          `json:"request_id,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// ListAdminAuditResult is a paginated admin_audit_log slice.
type ListAdminAuditResult struct {
	Items []AdminAuditRow `json:"items"`
	Page  int             `json:"page"`
	Limit int             `json:"limit"`
	Total int             `json:"total"`
}

// ListAdminAudit returns newest-first admin_audit_log rows with optional filters.
func (s Store) ListAdminAudit(ctx context.Context, action, entityType, q string, page, limit int) (ListAdminAuditResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	action = strings.TrimSpace(action)
	entityType = strings.TrimSpace(entityType)
	q = strings.TrimSpace(q)
	offset := (page - 1) * limit

	var total int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM admin_audit_log a
		LEFT JOIN admin_user u ON u.id = a.admin_user_id
		WHERE ($1 = '' OR a.action ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR a.entity_type = $2)
		  AND ($3 = '' OR
		       a.entity_id ILIKE '%' || $3 || '%' OR
		       COALESCE(u.email, '') ILIKE '%' || $3 || '%' OR
		       a.request_id ILIKE '%' || $3 || '%')`,
		action, entityType, q,
	).Scan(&total)
	if err != nil {
		return ListAdminAuditResult{}, fmt.Errorf("count admin audit: %w", err)
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT a.id, a.admin_user_id, COALESCE(u.email, ''),
		       a.action, a.entity_type, a.entity_id,
		       a.before_json, a.after_json, host(a.ip), a.ua, a.request_id, a.created_at
		FROM admin_audit_log a
		LEFT JOIN admin_user u ON u.id = a.admin_user_id
		WHERE ($1 = '' OR a.action ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR a.entity_type = $2)
		  AND ($3 = '' OR
		       a.entity_id ILIKE '%' || $3 || '%' OR
		       COALESCE(u.email, '') ILIKE '%' || $3 || '%' OR
		       a.request_id ILIKE '%' || $3 || '%')
		ORDER BY a.created_at DESC
		LIMIT $4 OFFSET $5`,
		action, entityType, q, limit, offset,
	)
	if err != nil {
		return ListAdminAuditResult{}, fmt.Errorf("list admin audit: %w", err)
	}
	defer rows.Close()

	items := make([]AdminAuditRow, 0)
	for rows.Next() {
		var (
			row      AdminAuditRow
			entityID *string
			ipStr    *string
			before   []byte
			after    []byte
		)
		if err := rows.Scan(
			&row.ID, &row.AdminUserID, &row.AdminEmail,
			&row.Action, &row.EntityType, &entityID,
			&before, &after, &ipStr, &row.UA, &row.RequestID, &row.CreatedAt,
		); err != nil {
			return ListAdminAuditResult{}, err
		}
		row.EntityID = entityID
		if len(before) > 0 {
			row.BeforeJSON = before
		}
		if len(after) > 0 {
			row.AfterJSON = after
		}
		row.IP = ipStr
		row.CreatedAt = row.CreatedAt.UTC()
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return ListAdminAuditResult{}, err
	}
	return ListAdminAuditResult{Items: items, Page: page, Limit: limit, Total: total}, nil
}
