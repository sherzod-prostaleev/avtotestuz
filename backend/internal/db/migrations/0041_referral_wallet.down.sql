DELETE FROM admin_role_permission
WHERE permission_id IN (
  SELECT id FROM admin_permission WHERE code IN (
    'referral.read', 'referral.payouts.manage', 'referral.rates.manage'
  )
);
DELETE FROM admin_permission
WHERE code IN ('referral.read', 'referral.payouts.manage', 'referral.rates.manage');

DELETE FROM limit_config WHERE key = 'referral_commission_percent_default';

ALTER TABLE referral_ledger DROP CONSTRAINT IF EXISTS referral_ledger_payout_fk;
DROP TABLE IF EXISTS referral_payout;
DROP TABLE IF EXISTS referral_ledger;

ALTER TABLE profile DROP COLUMN IF EXISTS referral_commission_percent;

DELETE FROM entitlement WHERE source = 'referral_buyer';

ALTER TABLE entitlement DROP CONSTRAINT entitlement_source_check;
ALTER TABLE entitlement ADD CONSTRAINT entitlement_source_check
  CHECK (source IN ('purchase','promo','referral','admin','b2b','trial'));
