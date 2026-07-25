-- Reverse of 0028: recreate the unused 0003-shaped table (empty; not used by app).
CREATE TABLE IF NOT EXISTS referral_attribution (
  referee_id    uuid PRIMARY KEY REFERENCES profile(id) ON DELETE CASCADE,
  referrer_id   uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  reward_status text NOT NULL DEFAULT 'pending'
                CHECK (reward_status IN ('pending','granted','rejected')),
  fraud_flags   jsonb NOT NULL DEFAULT '{}',
  created_at    timestamptz NOT NULL DEFAULT now()
);
