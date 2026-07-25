-- Feature flags (M3-7). Boolean/percentage/allowlist via type + value_json.
CREATE TABLE feature_flag (
  key         text PRIMARY KEY,
  type        text NOT NULL CHECK (type IN ('boolean', 'percentage', 'allowlist')),
  value_json  jsonb NOT NULL DEFAULT 'false'::jsonb,
  description text NOT NULL DEFAULT '',
  updated_by  text NOT NULL DEFAULT '',
  updated_at  timestamptz NOT NULL DEFAULT now()
);

INSERT INTO feature_flag (key, type, value_json, description) VALUES
  ('maintenance_mode', 'boolean', 'false'::jsonb, 'Global maintenance banner/gate (consumers opt-in later)'),
  ('arena_enabled', 'boolean', 'true'::jsonb, 'Battle Arena entry surfaces'),
  ('web_push_digest', 'boolean', 'true'::jsonb, 'Allow FSRS web-push digest sends when VAPID configured'),
  ('checkout_payme', 'boolean', 'true'::jsonb, 'Show Payme on checkout (provider kill-switch is separate)'),
  ('checkout_click', 'boolean', 'true'::jsonb, 'Show Click on checkout (provider kill-switch is separate)');
