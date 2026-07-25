-- Support inbox stub (M3 / U-47): short learner/public tickets, admin triage.
CREATE TABLE support_ticket (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id    uuid REFERENCES profile(id) ON DELETE SET NULL,
  contact_email text NOT NULL DEFAULT '',
  contact_phone text NOT NULL DEFAULT '',
  subject       text NOT NULL,
  body          text NOT NULL,
  status        text NOT NULL DEFAULT 'open'
                CHECK (status IN ('open', 'in_progress', 'resolved', 'closed')),
  locale        text NOT NULL DEFAULT 'uz-Latn',
  source        text NOT NULL DEFAULT 'profile'
                CHECK (source IN ('profile', 'public')),
  admin_note    text NOT NULL DEFAULT '',
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX support_ticket_status_created_idx
  ON support_ticket (status, created_at DESC);
CREATE INDEX support_ticket_profile_idx
  ON support_ticket (profile_id) WHERE profile_id IS NOT NULL;
