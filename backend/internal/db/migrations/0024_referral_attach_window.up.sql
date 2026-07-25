-- Referral attach window (days since profile.created_at).
-- free_value == vip_value: referral eligibility is not a VIP tier feature —
-- both paid and free referees use the same acquisition window. Tunable from
-- M3 admin later without a deploy.
INSERT INTO limit_config (key, free_value, vip_value) VALUES
  ('referral_attach_window_days', 30, 30);
