DELETE FROM admin_role_permission
WHERE permission_id IN (SELECT id FROM admin_permission WHERE code = 'content.verify');

DELETE FROM admin_permission WHERE code = 'content.verify';
