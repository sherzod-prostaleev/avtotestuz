DELETE FROM admin_role_permission
WHERE permission_id IN (
  SELECT id FROM admin_permission
  WHERE code IN ('users.hard_delete', 'b2b.orgs.hard_delete')
);

DELETE FROM admin_permission
WHERE code IN ('users.hard_delete', 'b2b.orgs.hard_delete');
