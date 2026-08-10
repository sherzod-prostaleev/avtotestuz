DROP TABLE IF EXISTS broadcast_recipient;

DROP INDEX IF EXISTS notification_profile_unread_idx;
DROP INDEX IF EXISTS notification_campaign_profile_inapp_uidx;

ALTER TABLE notification DROP COLUMN IF EXISTS campaign_id;

DROP TABLE IF EXISTS broadcast_campaign;
