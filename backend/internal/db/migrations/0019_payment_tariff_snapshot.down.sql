ALTER TABLE payment
  DROP CONSTRAINT IF EXISTS payment_tariff_days_snapshot_positive,
  DROP CONSTRAINT IF EXISTS payment_tariff_price_snapshot_nonneg,
  DROP COLUMN IF EXISTS tariff_days_snapshot,
  DROP COLUMN IF EXISTS tariff_price_uzs_snapshot;
