-- Refuse rollback while immutable void records exist: removing them would
-- destroy financial history. Operators must fix-forward instead.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM payment_void) THEN
    RAISE EXCEPTION 'cannot roll back 0047: payment_void contains immutable records';
  END IF;
END $$;

DROP TABLE payment_void;
DELETE FROM admin_role_permission
WHERE permission_id = (SELECT id FROM admin_permission WHERE code = 'payments.void');
DELETE FROM admin_permission WHERE code = 'payments.void';

ALTER TABLE payment DROP CONSTRAINT IF EXISTS payment_status_check;
ALTER TABLE payment ADD CONSTRAINT payment_status_check
  CHECK (status IN ('created','pending','paid','failed','canceled','refunded'));
