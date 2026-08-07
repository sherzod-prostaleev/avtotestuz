-- Re-gate classroom PCs behind sequential bilet progress.
--
-- Only station profiles are touched, and only back to the default: this
-- cannot reach a learner account, whose flag is set by ops for unrelated
-- reasons and is not what this migration ever changed.
UPDATE profile SET bypass_variant_progress = FALSE WHERE kind = 'station';
