-- Recreates the shape only. The rows are gone for good: this migration dropped
-- the tables that held them, and nothing archived the contents first.

ALTER TABLE b2b_org_license
  ADD COLUMN home_seats int NOT NULL DEFAULT 0 CHECK (home_seats >= 0);

CREATE TABLE b2b_org_member (
  org_id     uuid NOT NULL REFERENCES b2b_org(id) ON DELETE CASCADE,
  profile_id uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  role       text NOT NULL DEFAULT 'student'
             CHECK (role IN ('owner', 'teacher', 'student')),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, profile_id)
);
CREATE INDEX b2b_org_member_profile_idx ON b2b_org_member(profile_id);

CREATE TABLE b2b_invite (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  token       text NOT NULL UNIQUE,
  org_id      uuid NOT NULL REFERENCES b2b_org(id) ON DELETE CASCADE,
  phone       text NOT NULL,
  role        text NOT NULL DEFAULT 'student'
              CHECK (role IN ('owner', 'teacher', 'student')),
  expires_at  timestamptz NOT NULL,
  created_by  uuid REFERENCES profile(id) ON DELETE SET NULL,
  accepted_at timestamptz,
  accepted_by uuid REFERENCES profile(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  CHECK (expires_at > created_at)
);
CREATE INDEX b2b_invite_org_idx ON b2b_invite(org_id, created_at DESC);
CREATE INDEX b2b_invite_phone_pending_idx ON b2b_invite(phone)
  WHERE accepted_at IS NULL;
CREATE INDEX b2b_invite_token_pending_idx ON b2b_invite(token)
  WHERE accepted_at IS NULL;
