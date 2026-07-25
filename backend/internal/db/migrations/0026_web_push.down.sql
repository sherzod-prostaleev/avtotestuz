DELETE FROM notification WHERE channel = 'webpush';

ALTER TABLE notification DROP CONSTRAINT IF EXISTS notification_channel_check;
ALTER TABLE notification ADD CONSTRAINT notification_channel_check
  CHECK (channel IN ('inapp', 'telegram', 'sms'));

DROP TABLE IF EXISTS push_subscription;
