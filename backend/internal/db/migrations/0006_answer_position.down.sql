-- Revert to the original 1-4 range.
--
-- CAVEAT: if any answer rows with position = 5 have been inserted since the
-- .up migration ran (i.e. real 5-answer questions were imported), this ALTER
-- TABLE ... ADD CONSTRAINT will fail with a check-violation error, because
-- Postgres validates the new CHECK against existing data. That failure is
-- intentional/expected in that scenario: rolling back this migration is
-- only safe before any 5-answer data has been stored, or after such rows
-- have been manually removed/re-migrated. Do not force this down-migration
-- through by weakening the check further; decide what to do with the
-- 5-answer rows first.
ALTER TABLE answer DROP CONSTRAINT answer_position_check;
ALTER TABLE answer ADD CONSTRAINT answer_position_check CHECK (position BETWEEN 1 AND 4);
