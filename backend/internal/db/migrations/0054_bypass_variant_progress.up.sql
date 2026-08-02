-- Per-profile QA/ops flag: skip sequential bilet unlock (prev ticket
-- completed_at) while keeping VIP entitlement required for tickets #2+.
-- Default false — normal VIP progressive unlock is unchanged.
ALTER TABLE profile
  ADD COLUMN bypass_variant_progress boolean NOT NULL DEFAULT false;
