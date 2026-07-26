-- Hard-delete payment receipts from admin To'lovlar (absolute remove).
INSERT INTO admin_permission (code, description) VALUES
  ('payments.delete', 'Hard-delete payment transactions and related billing rows')
ON CONFLICT (code) DO NOTHING;

INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM admin_role r
JOIN admin_permission p ON p.code = 'payments.delete'
WHERE r.code IN ('superadmin', 'admin', 'finance')
ON CONFLICT DO NOTHING;
