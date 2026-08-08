-- Notification Center + Broadcast campaigns (in-app source of truth + optional web push).
-- Additive only: does not alter payment, entitlement, support_chat, or referral tables.

CREATE TABLE broadcast_campaign (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  created_by_admin   uuid NOT NULL REFERENCES admin_user(id),
  title              text NOT NULL,
  body               text NOT NULL,
  image_url          text NOT NULL DEFAULT '',
  action_url         text NOT NULL DEFAULT '',
  audience           text NOT NULL
                     CHECK (audience IN ('all_active', 'vip', 'non_vip')),
  channels           text NOT NULL
                     CHECK (channels IN ('inapp', 'both')),
  status             text NOT NULL DEFAULT 'queued'
                     CHECK (status IN (
                       'draft', 'queued', 'expanding', 'sending',
                       'completed', 'completed_with_errors', 'cancelled', 'failed'
                     )),
  idempotency_key    text UNIQUE,
  recipient_total    int NOT NULL DEFAULT 0 CHECK (recipient_total >= 0),
  pending_count      int NOT NULL DEFAULT 0 CHECK (pending_count >= 0),
  sent_count         int NOT NULL DEFAULT 0 CHECK (sent_count >= 0),
  failed_count       int NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
  push_sent_count    int NOT NULL DEFAULT 0 CHECK (push_sent_count >= 0),
  push_failed_count  int NOT NULL DEFAULT 0 CHECK (push_failed_count >= 0),
  error_summary      text NOT NULL DEFAULT '',
  created_at         timestamptz NOT NULL DEFAULT now(),
  queued_at          timestamptz,
  started_at         timestamptz,
  finished_at        timestamptz
);

CREATE INDEX broadcast_campaign_created_idx
  ON broadcast_campaign (created_at DESC);
CREATE INDEX broadcast_campaign_status_idx
  ON broadcast_campaign (status, created_at DESC);

ALTER TABLE notification
  ADD COLUMN campaign_id uuid NULL REFERENCES broadcast_campaign(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX notification_campaign_profile_inapp_uidx
  ON notification (campaign_id, profile_id)
  WHERE channel = 'inapp' AND campaign_id IS NOT NULL;

CREATE INDEX notification_profile_unread_idx
  ON notification (profile_id, created_at DESC)
  WHERE channel = 'inapp' AND read_at IS NULL;

CREATE TABLE broadcast_recipient (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id     uuid NOT NULL REFERENCES broadcast_campaign(id) ON DELETE CASCADE,
  profile_id      uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  status          text NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'processing', 'sent', 'failed')),
  attempt_count   int NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  last_error      text NOT NULL DEFAULT '',
  notification_id uuid REFERENCES notification(id) ON DELETE SET NULL,
  push_status     text NOT NULL DEFAULT 'pending'
                  CHECK (push_status IN ('skipped', 'pending', 'sent', 'failed', 'no_subscription')),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  processed_at    timestamptz,
  UNIQUE (campaign_id, profile_id)
);

CREATE INDEX broadcast_recipient_claim_idx
  ON broadcast_recipient (status, next_attempt_at)
  WHERE status IN ('pending', 'failed');

CREATE INDEX broadcast_recipient_campaign_status_idx
  ON broadcast_recipient (campaign_id, status);
