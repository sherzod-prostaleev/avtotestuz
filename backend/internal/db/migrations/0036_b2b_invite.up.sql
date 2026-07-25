-- B2B invite / enroll (M5-02). Phone-targeted pending join for schools.
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
