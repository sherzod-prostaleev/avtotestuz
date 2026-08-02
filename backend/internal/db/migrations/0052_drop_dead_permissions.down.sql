INSERT INTO admin_permission (code, description) VALUES
  ('users.hard_delete', 'Hard-delete learners'),
  ('payments.delete', 'Hard-delete payment transactions and related billing rows')
ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description;

-- users.hard_delete was only ever granted to superadmin, via 0030's blanket
-- CROSS JOIN over admin_permission (it was explicitly excluded from admin).
INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM admin_role r
JOIN admin_permission p ON p.code = 'users.hard_delete'
WHERE r.code = 'superadmin'
ON CONFLICT DO NOTHING;

-- payments.delete was granted to superadmin, admin, finance (0045).
INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM admin_role r
JOIN admin_permission p ON p.code = 'payments.delete'
WHERE r.code IN ('superadmin', 'admin', 'finance')
ON CONFLICT DO NOTHING;
