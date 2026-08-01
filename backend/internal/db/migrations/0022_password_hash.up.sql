-- Learner auth moves from OTP to phone + password. Existing OTP-era rows keep
-- a NULL hash. Password recovery must prove phone ownership; the historical
-- unauthenticated set-password endpoint was removed.
ALTER TABLE profile
  ADD COLUMN password_hash text;
