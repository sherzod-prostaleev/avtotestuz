-- Referral cash wallet: per-user commission %, append-only ledger, payouts.
-- Buyer bonus days use entitlement source referral_buyer (referrer no longer gets VIP days).

ALTER TABLE entitlement DROP CONSTRAINT entitlement_source_check;
ALTER TABLE entitlement ADD CONSTRAINT entitlement_source_check
  CHECK (source IN ('purchase','promo','referral','referral_buyer','admin','b2b','trial'));

ALTER TABLE profile
  ADD COLUMN referral_commission_percent int NOT NULL DEFAULT 20
    CHECK (referral_commission_percent >= 0 AND referral_commission_percent <= 100);

CREATE TABLE referral_ledger (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id  uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  entry_type  text NOT NULL
              CHECK (entry_type IN (
                'commission',
                'payout_hold',
                'payout_paid',
                'payout_reject_release',
                'clawback'
              )),
  amount_uzs  bigint NOT NULL,
  payment_id  uuid REFERENCES payment(id),
  payout_id   uuid,
  meta        jsonb NOT NULL DEFAULT '{}',
  created_at  timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT referral_ledger_amount_nonzero CHECK (amount_uzs <> 0)
);

-- One commission credit per paid payment (idempotent webhooks).
CREATE UNIQUE INDEX referral_ledger_commission_payment_uidx
  ON referral_ledger (payment_id)
  WHERE entry_type = 'commission' AND payment_id IS NOT NULL;

-- One clawback per payment.
CREATE UNIQUE INDEX referral_ledger_clawback_payment_uidx
  ON referral_ledger (payment_id)
  WHERE entry_type = 'clawback' AND payment_id IS NOT NULL;

CREATE INDEX referral_ledger_profile_idx
  ON referral_ledger (profile_id, created_at DESC);

CREATE TABLE referral_payout (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id    uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  amount_uzs    bigint NOT NULL CHECK (amount_uzs > 0),
  card_number   text NOT NULL,
  card_network  text NOT NULL CHECK (card_network IN ('uzcard', 'humo')),
  status        text NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'paid', 'rejected')),
  admin_note    text NOT NULL DEFAULT '',
  processed_by  uuid REFERENCES admin_user(id) ON DELETE SET NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  processed_at  timestamptz
);

ALTER TABLE referral_ledger
  ADD CONSTRAINT referral_ledger_payout_fk
  FOREIGN KEY (payout_id) REFERENCES referral_payout(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX referral_ledger_payout_hold_uidx
  ON referral_ledger (payout_id)
  WHERE entry_type = 'payout_hold' AND payout_id IS NOT NULL;

CREATE INDEX referral_payout_status_idx
  ON referral_payout (status, created_at DESC);

CREATE INDEX referral_payout_profile_idx
  ON referral_payout (profile_id, created_at DESC);

INSERT INTO limit_config (key, free_value, vip_value)
VALUES ('referral_commission_percent_default', 20, 20)
ON CONFLICT (key) DO NOTHING;

INSERT INTO admin_permission (code, description) VALUES
  ('referral.read', 'Read referral stats and payout queue'),
  ('referral.payouts.manage', 'Approve or reject referral payouts'),
  ('referral.rates.manage', 'Edit per-user referral commission percent')
ON CONFLICT (code) DO NOTHING;

-- superadmin already has all via cross join at seed time; grant to existing roles.
INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM admin_role r
JOIN admin_permission p ON p.code IN (
  'referral.read', 'referral.payouts.manage', 'referral.rates.manage'
)
WHERE r.code = 'superadmin'
ON CONFLICT DO NOTHING;

INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM admin_role r
JOIN admin_permission p ON p.code IN (
  'referral.read', 'referral.payouts.manage', 'referral.rates.manage'
)
WHERE r.code = 'admin'
ON CONFLICT DO NOTHING;

INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM admin_role r
JOIN admin_permission p ON p.code IN ('referral.read', 'referral.payouts.manage')
WHERE r.code = 'finance'
ON CONFLICT DO NOTHING;

INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM admin_role r
JOIN admin_permission p ON p.code = 'referral.read'
WHERE r.code = 'support'
ON CONFLICT DO NOTHING;
