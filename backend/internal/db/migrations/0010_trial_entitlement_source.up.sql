-- New profiles now receive a short automatic entitlement, which is neither a
-- purchase nor a promo redemption: recording it as either would corrupt
-- revenue reporting and promo-usage counts. It gets its own source so trials
-- can be separated from paid entitlements in any later analysis.
ALTER TABLE entitlement DROP CONSTRAINT entitlement_source_check;
ALTER TABLE entitlement ADD CONSTRAINT entitlement_source_check
  CHECK (source IN ('purchase','promo','referral','admin','b2b','trial'));
