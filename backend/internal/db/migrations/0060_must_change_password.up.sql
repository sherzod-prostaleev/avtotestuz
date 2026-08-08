-- Force password change after admin issues a temporary password.
-- Default false keeps existing learners unaffected (backwards-compatible).
ALTER TABLE profile
  ADD COLUMN must_change_password boolean NOT NULL DEFAULT false;
