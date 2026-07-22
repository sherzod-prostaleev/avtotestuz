CREATE TABLE session_question (
  session_id  uuid NOT NULL REFERENCES exam_session(id) ON DELETE CASCADE,
  question_id uuid NOT NULL REFERENCES question(id),
  position    smallint NOT NULL CHECK (position >= 1),
  PRIMARY KEY (session_id, question_id),
  UNIQUE (session_id, position)
);

-- Historical sessions did not retain their unanswered question set. Preserve
-- every answer that can be recovered and give it a deterministic position so
-- the new composite FK can be added safely even if legacy submissions raced.
INSERT INTO session_question (session_id, question_id, position)
SELECT
  session_id,
  question_id,
  row_number() OVER (
    PARTITION BY session_id
    ORDER BY position, answered_at, question_id
  )::smallint
FROM session_answer;

ALTER TABLE session_answer
  ADD CONSTRAINT session_answer_session_question_fk
  FOREIGN KEY (session_id, question_id)
  REFERENCES session_question(session_id, question_id);
