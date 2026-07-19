-- Widen answer.position from 1-4 to 1-5: the real licensed dataset (1235
-- questions) includes 119 questions with 5 answers. The importer previously
-- silently dropped any answer at position 5 (see store.go), which for 25 of
-- those questions meant dropping the CORRECT answer entirely — a question
-- stored with zero correct answers. The validator already accepts 2-5
-- answers; this migration brings the DB constraint in line with it.
ALTER TABLE answer DROP CONSTRAINT answer_position_check;
ALTER TABLE answer ADD CONSTRAINT answer_position_check CHECK (position BETWEEN 1 AND 5);
