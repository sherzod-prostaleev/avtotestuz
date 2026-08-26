-- Remember how far a profile has worked through a topic, so Practice ->
-- Category -> "Hammasi" can be an ordered walk that resumes instead of a fresh
-- random draw every time.
--
-- Per profile, per topic. On a classroom PC the profile is the station's
-- shadow profile, so this is the room's position rather than any one
-- student's: the teacher says "topic 5, all questions", the class reaches 123
-- today and starts at 124 tomorrow. That is deliberate -- a station has one
-- profile and nothing that could tell one student from another.
--
-- next_index is 0-based and counts questions worked through, so 123 means the
-- next question is the 124th. It is bounded by the topic's question count at
-- read time rather than by a constraint here: the bank can shrink (a question
-- quarantined) and a cursor left past the end must wrap, not fail an insert.
CREATE TABLE practice_cursor (
  profile_id  uuid        NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  category_id uuid        NOT NULL REFERENCES category(id) ON DELETE CASCADE,
  next_index  int         NOT NULL DEFAULT 0 CHECK (next_index >= 0),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (profile_id, category_id)
);

-- Which index an ordered session started at, so recording an answer can move
-- the cursor to the right absolute position. NULL on every session that is not
-- an ordered walk, which is all of them today and most of them after this:
-- exams, mistakes, placement, signs, ticket ranges, and every practice draw
-- that is not "all questions of one topic".
ALTER TABLE exam_session ADD COLUMN ordered_from int CHECK (ordered_from >= 0);
