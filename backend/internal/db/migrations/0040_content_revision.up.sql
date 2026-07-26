-- M3-2a: immutable content snapshots on admin mutations (PRD content_revision).
CREATE TABLE content_revision (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type text NOT NULL,
  entity_id uuid NOT NULL,
  snapshot_json jsonb NOT NULL,
  editor_id uuid REFERENCES admin_user (id) ON DELETE SET NULL,
  note text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX content_revision_entity_idx
  ON content_revision (entity_type, entity_id, created_at DESC);
