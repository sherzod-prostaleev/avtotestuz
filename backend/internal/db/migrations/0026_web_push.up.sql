-- M4-08 / U-11: Web Push subscription store + notification channel.
-- See docs/superpowers/specs/2026-07-26-m4-08-web-push-design.md.

CREATE TABLE push_subscription (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id  uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  endpoint    text NOT NULL UNIQUE,
  p256dh      text NOT NULL,
  auth        text NOT NULL,
  user_agent  text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now(),
  last_seen   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX push_subscription_profile_idx ON push_subscription(profile_id);

ALTER TABLE notification DROP CONSTRAINT IF EXISTS notification_channel_check;
ALTER TABLE notification ADD CONSTRAINT notification_channel_check
  CHECK (channel IN ('inapp', 'telegram', 'sms', 'webpush'));
