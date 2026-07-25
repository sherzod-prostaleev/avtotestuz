-- Runtime kill-switches for Payme/Click. Prod merchant keys stay in env;
-- this table only controls whether new checkouts may start. In-flight
-- webhooks still complete so users who already paid are not stranded.
CREATE TABLE payment_provider_status (
  provider text PRIMARY KEY CHECK (provider IN ('payme', 'click')),
  enabled boolean NOT NULL DEFAULT true,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by text
);

INSERT INTO payment_provider_status (provider, enabled) VALUES
  ('payme', true),
  ('click', true);
