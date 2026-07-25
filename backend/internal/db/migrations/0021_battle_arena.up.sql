-- M4-03 Battle Arena tables (VIP-only 1v1 duel). Separate from exam_session
-- so timed-out answers (answer_id NULL) and leaderboard isolation work by default.
CREATE TABLE arena_match (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  status            text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','in_progress','finished','aborted')),
  question_ids      uuid[] NOT NULL CHECK (cardinality(question_ids) > 0),
  question_time_sec smallint NOT NULL CHECK (question_time_sec > 0),
  created_at        timestamptz NOT NULL DEFAULT now(),
  started_at        timestamptz,
  finished_at       timestamptz,
  end_reason        text CHECK (end_reason IS NULL OR end_reason IN
                    ('completed','forfeit','both_disconnected','server_shutdown'))
);

CREATE TABLE arena_match_player (
  match_id          uuid NOT NULL REFERENCES arena_match(id) ON DELETE CASCADE,
  profile_id        uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  slot              smallint NOT NULL CHECK (slot IN (1,2)),
  locale            locale_code NOT NULL,
  score             int      NOT NULL DEFAULT 0,
  correct_count     smallint NOT NULL DEFAULT 0,
  total_response_ms int      NOT NULL DEFAULT 0,
  outcome           text CHECK (outcome IS NULL OR outcome IN ('won','lost','draw')),
  rating_before     int,
  rating_after      int,
  rating_delta      int,
  disconnected_at   timestamptz,
  joined_at         timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (match_id, profile_id),
  UNIQUE (match_id, slot)
);

CREATE INDEX arena_match_player_profile_idx ON arena_match_player(profile_id, joined_at DESC);

CREATE TABLE arena_answer (
  match_id    uuid NOT NULL REFERENCES arena_match(id) ON DELETE CASCADE,
  profile_id  uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  question_id uuid NOT NULL REFERENCES question(id),
  position    smallint NOT NULL,
  answer_id   uuid REFERENCES answer(id),
  is_correct  boolean NOT NULL DEFAULT false,
  response_ms int,
  points      smallint NOT NULL DEFAULT 0,
  answered_at timestamptz,
  PRIMARY KEY (match_id, profile_id, question_id),
  CHECK ((answer_id IS NULL) = (response_ms IS NULL)),
  CHECK (answer_id IS NOT NULL OR NOT is_correct)
);
