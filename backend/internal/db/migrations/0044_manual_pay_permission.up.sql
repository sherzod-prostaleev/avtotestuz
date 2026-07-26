-- Manual Humo admin ops: separate from payments.keys.manage (Payme/Click secrets).
INSERT INTO admin_permission (code, description) VALUES
  ('payments.manual.manage', 'Manage manual Humo cards, queue, and HUMOcardbot userbot')
ON CONFLICT (code) DO NOTHING;

INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM admin_role r
JOIN admin_permission p ON p.code = 'payments.manual.manage'
WHERE r.code IN ('superadmin', 'admin', 'finance')
ON CONFLICT DO NOTHING;
