CREATE TABLE audit_log (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id   uuid REFERENCES profile(id),
  action     text NOT NULL,
  entity     text NOT NULL,
  entity_id  uuid,
  before     jsonb,
  after      jsonb,
  ip         inet,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_entity_idx ON audit_log(entity, entity_id, created_at DESC);

CREATE TABLE event (
  id         bigint GENERATED ALWAYS AS IDENTITY,
  profile_id uuid,
  anon_id    text,
  name       text NOT NULL,
  props      jsonb NOT NULL DEFAULT '{}',
  ts         timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, ts)
) PARTITION BY RANGE (ts);
CREATE TABLE event_default PARTITION OF event DEFAULT;
CREATE INDEX event_name_ts_idx ON event(name, ts);

CREATE TABLE notification (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  kind       text NOT NULL,
  payload    jsonb NOT NULL DEFAULT '{}',
  channel    text NOT NULL CHECK (channel IN ('inapp','telegram','sms')),
  sent_at    timestamptz,
  read_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notification_profile_idx ON notification(profile_id, created_at DESC);
