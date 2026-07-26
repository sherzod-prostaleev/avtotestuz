package admin

import (
	"context"
	"fmt"
)

// RBACPermission is one admin_permission row for the matrix header.
type RBACPermission struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// RBACRole is one admin_role with granted permission codes.
type RBACRole struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// RBACMatrix is GET /admin/v1/security/rbac (read-only).
type RBACMatrix struct {
	Roles       []RBACRole       `json:"roles"`
	Permissions []RBACPermission `json:"permissions"`
}

// GetRBACMatrix returns all roles, permissions, and role→permission grants.
func (s Store) GetRBACMatrix(ctx context.Context) (RBACMatrix, error) {
	permRows, err := s.Pool.Query(ctx, `
		SELECT code, description
		FROM admin_permission
		ORDER BY code`)
	if err != nil {
		return RBACMatrix{}, fmt.Errorf("list admin permissions: %w", err)
	}
	defer permRows.Close()

	perms := make([]RBACPermission, 0)
	for permRows.Next() {
		var p RBACPermission
		if err := permRows.Scan(&p.Code, &p.Description); err != nil {
			return RBACMatrix{}, err
		}
		perms = append(perms, p)
	}
	if err := permRows.Err(); err != nil {
		return RBACMatrix{}, err
	}

	roleRows, err := s.Pool.Query(ctx, `
		SELECT r.code, r.name, r.description, COALESCE(array_agg(p.code ORDER BY p.code) FILTER (WHERE p.code IS NOT NULL), '{}')
		FROM admin_role r
		LEFT JOIN admin_role_permission rp ON rp.role_id = r.id
		LEFT JOIN admin_permission p ON p.id = rp.permission_id
		GROUP BY r.id, r.code, r.name, r.description
		ORDER BY r.code`)
	if err != nil {
		return RBACMatrix{}, fmt.Errorf("list admin roles: %w", err)
	}
	defer roleRows.Close()

	roles := make([]RBACRole, 0)
	for roleRows.Next() {
		var role RBACRole
		if err := roleRows.Scan(&role.Code, &role.Name, &role.Description, &role.Permissions); err != nil {
			return RBACMatrix{}, err
		}
		if role.Permissions == nil {
			role.Permissions = []string{}
		}
		roles = append(roles, role)
	}
	if err := roleRows.Err(); err != nil {
		return RBACMatrix{}, err
	}

	return RBACMatrix{Roles: roles, Permissions: perms}, nil
}
