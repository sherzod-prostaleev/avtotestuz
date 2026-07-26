DROP TABLE IF EXISTS manual_pay_event;
DROP TABLE IF EXISTS manual_pay_assignment;
DROP TABLE IF EXISTS manual_pay_card;
DROP TABLE IF EXISTS manual_tg_settings;

DELETE FROM feature_flag WHERE key = 'checkout_manual';
DELETE FROM payment_provider_status WHERE provider = 'manual';

ALTER TABLE payment_provider_status DROP CONSTRAINT IF EXISTS payment_provider_status_provider_check;
ALTER TABLE payment_provider_status ADD CONSTRAINT payment_provider_status_provider_check
  CHECK (provider IN ('payme', 'click'));

-- Only safe if no manual payments remain.
ALTER TABLE payment DROP CONSTRAINT IF EXISTS payment_provider_check;
ALTER TABLE payment ADD CONSTRAINT payment_provider_check
  CHECK (provider IN ('payme', 'click', 'sandbox'));
