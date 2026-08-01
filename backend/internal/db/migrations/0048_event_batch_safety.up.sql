CREATE TABLE event_batch (
  profile_id uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  idempotency_key uuid NOT NULL,
  event_count integer NOT NULL CHECK (event_count BETWEEN 1 AND 100),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (profile_id, idempotency_key)
);

CREATE INDEX event_batch_created_idx ON event_batch (created_at);
