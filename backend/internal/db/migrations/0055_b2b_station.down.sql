DROP TABLE IF EXISTS b2b_station;
DROP TABLE IF EXISTS b2b_station_activate_code;

DROP INDEX IF EXISTS promo_code_partner_org_idx;
ALTER TABLE promo_code DROP COLUMN IF EXISTS partner_org_id;

ALTER TABLE b2b_org_license DROP COLUMN IF EXISTS home_seats;
