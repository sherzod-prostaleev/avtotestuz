-- U-35: persist Grand Mock pass certificates with a shareable public code.
-- Confetti UI alone is not a credential; this stores a durable, shareable id.

CREATE TABLE grand_mock_certificate (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id  uuid NOT NULL UNIQUE REFERENCES exam_session(id) ON DELETE CASCADE,
  profile_id  uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  share_code  text NOT NULL UNIQUE,
  score       int  NOT NULL CHECK (score >= 0),
  total       int  NOT NULL CHECK (total > 0),
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX grand_mock_certificate_profile_idx
  ON grand_mock_certificate(profile_id, created_at DESC);
