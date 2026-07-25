-- M3-2: content.verify for explanation queue mark-verified (CLI path already existed).
INSERT INTO admin_permission (code, description) VALUES
  ('content.verify', 'Verify explanations and content review actions')
ON CONFLICT (code) DO NOTHING;

-- superadmin gets every new permission
INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM admin_role r
CROSS JOIN admin_permission p
WHERE r.code = 'superadmin' AND p.code = 'content.verify'
ON CONFLICT DO NOTHING;

-- admin + editor (content studio operators)
INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM admin_role r
JOIN admin_permission p ON p.code = 'content.verify'
WHERE r.code IN ('admin', 'editor')
ON CONFLICT DO NOTHING;
