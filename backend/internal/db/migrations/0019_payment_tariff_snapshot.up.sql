-- Freeze the tariff terms onto the payment row at checkout time.
--
-- A payment previously recorded only amount_uzs (what was charged) and a
-- tariff_id, so everything downstream re-read tariff.days / tariff.price_uzs
-- from the live row. Those are mutable, and a 'created' payment never expires,
-- so the gap between checkout and webhook completion is unbounded: editing a
-- tariff's price or term silently rewrites what already-started purchases are
-- worth. Concretely, a customer who paid the full list price would receive
-- fewer days after a price increase, and ProcessPaymentGrant's pro-rating
-- would divide a frozen amount by a moved price.
--
-- The snapshot is what the customer was actually sold, so it is the only
-- correct basis for granting days, pro-rating, and payment history.

ALTER TABLE payment
  ADD COLUMN tariff_days_snapshot      int,
  ADD COLUMN tariff_price_uzs_snapshot bigint;

-- Existing rows predate the snapshot; the live tariff is the only information
-- available for them and is what the old code would have used anyway.
UPDATE payment p
SET tariff_days_snapshot      = t.days,
    tariff_price_uzs_snapshot = t.price_uzs
FROM tariff t
WHERE t.id = p.tariff_id
  AND p.tariff_days_snapshot IS NULL;

-- NOT NULL from here on: a payment that does not record what it sold is not a
-- usable record, and a required column makes that impossible to forget on a
-- future insert path rather than something a reviewer has to catch.
ALTER TABLE payment
  ALTER COLUMN tariff_days_snapshot      SET NOT NULL,
  ALTER COLUMN tariff_price_uzs_snapshot SET NOT NULL,
  ADD CONSTRAINT payment_tariff_days_snapshot_positive
    CHECK (tariff_days_snapshot > 0),
  ADD CONSTRAINT payment_tariff_price_snapshot_nonneg
    CHECK (tariff_price_uzs_snapshot >= 0);
