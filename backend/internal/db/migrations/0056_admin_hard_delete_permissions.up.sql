-- Destructive profile/org deletion is intentionally superadmin-only.
-- users.hard_delete existed in 0030 but was removed as dead surface in 0052;
-- it is safe to restore now that the API and two-step UI exist.
INSERT INTO admin_permission (code, description) VALUES
  ('users.hard_delete', 'Permanently delete a learner and directly related data'),
  ('b2b.orgs.hard_delete', 'Permanently delete a B2B organization and directly related data')
ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM admin_role r
JOIN admin_permission p ON p.code IN ('users.hard_delete', 'b2b.orgs.hard_delete')
WHERE r.code = 'superadmin'
ON CONFLICT DO NOTHING;
