-- Public site chrome settings (footer contacts, etc.). Ops edits via
-- OPS_ADMIN_TOKEN; public GET returns non-secret contact fields only.
CREATE TABLE site_settings (
  key text PRIMARY KEY,
  value_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by text
);

INSERT INTO site_settings (key, value_json) VALUES
  ('contacts', '{}'::jsonb);
