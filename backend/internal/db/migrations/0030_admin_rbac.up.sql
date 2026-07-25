-- M3-0 Super Admin foundation: staff identity, RBAC, sessions, audit.
-- Separate from learner profile / audit_log (profile FK).

CREATE TABLE admin_user (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email           text NOT NULL UNIQUE,
  phone           text,
  display_name    text NOT NULL DEFAULT '',
  password_hash   text NOT NULL,
  status          text NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active', 'disabled')),
  totp_secret_enc text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE admin_role (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code        text NOT NULL UNIQUE,
  name        text NOT NULL,
  description text NOT NULL DEFAULT ''
);

CREATE TABLE admin_permission (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code        text NOT NULL UNIQUE,
  description text NOT NULL DEFAULT ''
);

CREATE TABLE admin_role_permission (
  role_id       uuid NOT NULL REFERENCES admin_role(id) ON DELETE CASCADE,
  permission_id uuid NOT NULL REFERENCES admin_permission(id) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE admin_user_role (
  admin_user_id uuid NOT NULL REFERENCES admin_user(id) ON DELETE CASCADE,
  role_id       uuid NOT NULL REFERENCES admin_role(id) ON DELETE CASCADE,
  PRIMARY KEY (admin_user_id, role_id)
);

CREATE TABLE admin_session (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_user_id uuid NOT NULL REFERENCES admin_user(id) ON DELETE CASCADE,
  refresh_hash  text NOT NULL UNIQUE,
  ip            inet,
  ua            text NOT NULL DEFAULT '',
  expires_at    timestamptz NOT NULL,
  revoked_at    timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX admin_session_user_idx ON admin_session(admin_user_id)
  WHERE revoked_at IS NULL;

CREATE TABLE admin_audit_log (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_user_id uuid REFERENCES admin_user(id) ON DELETE SET NULL,
  action        text NOT NULL,
  entity_type   text NOT NULL,
  entity_id     text,
  before_json   jsonb,
  after_json    jsonb,
  ip            inet,
  ua            text NOT NULL DEFAULT '',
  request_id    text NOT NULL DEFAULT '',
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX admin_audit_log_created_idx ON admin_audit_log(created_at DESC);
CREATE INDEX admin_audit_log_entity_idx ON admin_audit_log(entity_type, entity_id, created_at DESC);

-- Seed roles
INSERT INTO admin_role (code, name, description) VALUES
  ('superadmin', 'Superadmin', 'All permissions'),
  ('admin', 'Admin', 'Ops + content + users + billing refund'),
  ('editor', 'Editor', 'Content CRUD'),
  ('support', 'Support', 'Users read/block + support inbox'),
  ('finance', 'Finance', 'Payments read/refund/export'),
  ('investor_viewer', 'Investor viewer', 'Investors + analytics read-only'),
  ('analyst', 'Analyst', 'Analytics + exports');

-- Seed permission keys (matrix subset for M3-0/M3-1)
INSERT INTO admin_permission (code, description) VALUES
  ('monitoring.read', 'Read system health and metrics'),
  ('monitoring.alerts.manage', 'Manage alert rules'),
  ('users.read', 'Read user directory and detail'),
  ('users.write', 'Update user profile fields'),
  ('users.block', 'Block or unblock learners'),
  ('users.hard_delete', 'Hard-delete learners'),
  ('users.sessions.revoke', 'Revoke learner sessions'),
  ('users.entitlements.grant', 'Grant entitlements'),
  ('content.questions.read', 'Read questions'),
  ('content.questions.write', 'Edit questions'),
  ('content.publish', 'Publish content'),
  ('payments.read', 'Read payments'),
  ('payments.refund', 'Initiate refunds'),
  ('payments.keys.manage', 'Manage provider keys'),
  ('payments.catalog.write', 'Edit tariffs/promo'),
  ('cms.read', 'Read CMS surfaces'),
  ('cms.write', 'Edit CMS surfaces'),
  ('analytics.read', 'Read analytics'),
  ('analytics.export', 'Export analytics'),
  ('settings.flags', 'Manage feature flags'),
  ('settings.config', 'Manage runtime config'),
  ('settings.maintenance', 'Maintenance mode'),
  ('security.rbac', 'Manage admins and RBAC'),
  ('security.audit.read', 'Read admin audit log'),
  ('support.inbox', 'Support inbox'),
  ('support.broadcast', 'Send broadcasts'),
  ('investors.read', 'Read investors');

-- superadmin: all permissions
INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM admin_role r CROSS JOIN admin_permission p
WHERE r.code = 'superadmin';

-- admin: broad ops (no hard_delete, no payments.keys.manage)
INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM admin_role r
JOIN admin_permission p ON p.code IN (
  'monitoring.read', 'monitoring.alerts.manage',
  'users.read', 'users.write', 'users.block', 'users.sessions.revoke', 'users.entitlements.grant',
  'content.questions.read', 'content.questions.write', 'content.publish',
  'payments.read', 'payments.refund', 'payments.catalog.write',
  'cms.read', 'cms.write',
  'analytics.read', 'analytics.export',
  'settings.flags', 'settings.config', 'settings.maintenance',
  'security.audit.read',
  'support.inbox', 'support.broadcast'
) WHERE r.code = 'admin';

-- editor
INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM admin_role r
JOIN admin_permission p ON p.code IN (
  'content.questions.read', 'content.questions.write', 'content.publish', 'cms.read'
) WHERE r.code = 'editor';

-- support
INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM admin_role r
JOIN admin_permission p ON p.code IN (
  'users.read', 'users.block', 'users.sessions.revoke', 'support.inbox', 'payments.read'
) WHERE r.code = 'support';

-- finance
INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM admin_role r
JOIN admin_permission p ON p.code IN (
  'payments.read', 'payments.refund', 'payments.catalog.write', 'analytics.read', 'analytics.export'
) WHERE r.code = 'finance';

-- investor_viewer
INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM admin_role r
JOIN admin_permission p ON p.code IN (
  'investors.read', 'analytics.read'
) WHERE r.code = 'investor_viewer';

-- analyst
INSERT INTO admin_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM admin_role r
JOIN admin_permission p ON p.code IN (
  'analytics.read', 'analytics.export', 'monitoring.read'
) WHERE r.code = 'analyst';
