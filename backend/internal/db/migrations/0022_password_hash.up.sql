-- Learner auth moves from OTP to phone + password. Existing OTP-era rows keep
-- a NULL hash; login returns password_not_set and set-password can fill it once.
ALTER TABLE profile
  ADD COLUMN password_hash text;
