-- B2B orgs / seats / license (U-40 M5-01 thin slice).
CREATE TABLE b2b_org (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name       text NOT NULL,
  status     text NOT NULL DEFAULT 'active'
             CHECK (status IN ('active', 'suspended')),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE b2b_org_member (
  org_id     uuid NOT NULL REFERENCES b2b_org(id) ON DELETE CASCADE,
  profile_id uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  role       text NOT NULL DEFAULT 'student'
             CHECK (role IN ('owner', 'teacher', 'student')),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, profile_id)
);
CREATE INDEX b2b_org_member_profile_idx ON b2b_org_member(profile_id);

CREATE TABLE b2b_org_license (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     uuid NOT NULL REFERENCES b2b_org(id) ON DELETE CASCADE,
  seats      int NOT NULL CHECK (seats > 0),
  starts_at  timestamptz NOT NULL,
  ends_at    timestamptz NOT NULL,
  note       text NOT NULL DEFAULT '',
  created_by text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (ends_at > starts_at)
);
CREATE INDEX b2b_org_license_org_idx ON b2b_org_license(org_id, ends_at DESC);
