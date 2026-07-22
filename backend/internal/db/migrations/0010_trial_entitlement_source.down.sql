-- Trial rows must go before the constraint can exclude them again.
DELETE FROM entitlement WHERE source = 'trial';
ALTER TABLE entitlement DROP CONSTRAINT entitlement_source_check;
ALTER TABLE entitlement ADD CONSTRAINT entitlement_source_check
  CHECK (source IN ('purchase','promo','referral','admin','b2b'));
