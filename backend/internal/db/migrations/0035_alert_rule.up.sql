-- Thin alert_rule catalog for admin monitoring (M3 / U-41). Rules are evaluated live — not a pager.
CREATE TABLE alert_rule (
  id          text PRIMARY KEY,
  name        text NOT NULL,
  kind        text NOT NULL
              CHECK (kind IN ('postgres_ready', 'payment_fails_24h')),
  enabled     boolean NOT NULL DEFAULT true,
  description text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now()
);

INSERT INTO alert_rule (id, name, kind, enabled, description) VALUES
  ('postgres_ready', 'Postgres readiness', 'postgres_ready', true,
   'Fires when admin health cannot ping Postgres.'),
  ('payment_fails_24h', 'Payment fails (24h)', 'payment_fails_24h', true,
   'Warns when ≥1 payment has status=failed in the last 24 hours.');
