DELETE FROM admin_role_permission
WHERE permission_id IN (SELECT id FROM admin_permission WHERE code = 'payments.delete');

DELETE FROM admin_permission WHERE code = 'payments.delete';
