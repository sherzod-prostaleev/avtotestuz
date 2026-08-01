-- Immutable payment voids replace destructive payment deletion. Historical
-- provider, entitlement, referral and assignment rows are intentionally kept.
ALTER TABLE payment DROP CONSTRAINT IF EXISTS payment_status_check;
ALTER TABLE payment ADD CONSTRAINT payment_status_check
  CHECK (status IN ('created','pending','paid','failed','canceled','refunded','voided'));

CREATE TABLE payment_void (
  payment_id uuid PRIMARY KEY REFERENCES payment(id),
  previous_status text NOT NULL,
  reason text NOT NULL CHECK (char_length(trim(reason)) >= 10),
  requested_by uuid NOT NULL REFERENCES admin_user(id),
  created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO admin_permission (code, description) VALUES
  ('payments.void', 'Void terminal unsuccessful or manual payments without deleting financial history')
ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM admin_role r
JOIN admin_permission p ON p.code = 'payments.void'
WHERE r.code IN ('superadmin', 'admin', 'finance')
ON CONFLICT DO NOTHING;

COMMENT ON COLUMN manual_pay_card.pan_full IS
  'AES-GCM envelope enc:v1:<last4>:<ciphertext>; legacy plaintext is migrated by cmd/encryptpan';
COMMENT ON COLUMN referral_payout.card_number IS
  'AES-GCM envelope enc:v1:<last4>:<ciphertext>; legacy plaintext is migrated by cmd/encryptpan';
