-- AD-5: users.hard_delete (0030) and payments.delete (0045) are seeded and
-- granted to roles but have no implementation anywhere in Go code — no
-- RequirePermission call, no handler, nothing — so the RBAC matrix
-- (GET /admin/v1/security/rbac) advertises privileges that do nothing.
-- payments.delete was also superseded by the immutable payments.void
-- (0047): payment history is voided, never hard-deleted. Drop the grants and
-- the permission rows themselves; the down migration restores both exactly
-- as 0030/0045 left them.
DELETE FROM admin_role_permission
WHERE permission_id IN (
  SELECT id FROM admin_permission WHERE code IN ('users.hard_delete', 'payments.delete')
);

DELETE FROM admin_permission WHERE code IN ('users.hard_delete', 'payments.delete');
