CREATE TABLE exam_session (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id     uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  mode           text NOT NULL
                 CHECK (mode IN ('variant','exam','practice','mistakes','grand_mock')),
  variant_id     uuid REFERENCES variant(id),
  category_id    uuid REFERENCES category(id),
  sign_id        uuid REFERENCES sign(id),
  locale         locale_code NOT NULL,
  time_limit_sec int,
  errors_allowed int,
  started_at     timestamptz NOT NULL DEFAULT now(),
  finished_at    timestamptz,
  status         text NOT NULL DEFAULT 'in_progress'
                 CHECK (status IN ('in_progress','passed','failed','abandoned')),
  score          int,
  total          int NOT NULL,
  stopped_reason text CHECK (stopped_reason IS NULL OR
                 stopped_reason IN ('completed','time_up','too_many_errors'))
);
CREATE INDEX exam_session_profile_idx ON exam_session(profile_id, started_at DESC);

CREATE TABLE session_answer (
  session_id  uuid NOT NULL REFERENCES exam_session(id) ON DELETE CASCADE,
  question_id uuid NOT NULL REFERENCES question(id),
  answer_id   uuid NOT NULL REFERENCES answer(id),
  is_correct  boolean NOT NULL,
  position    smallint NOT NULL,
  answered_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (session_id, question_id)
);

CREATE TABLE variant_progress (
  profile_id   uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  variant_id   uuid NOT NULL REFERENCES variant(id) ON DELETE CASCADE,
  best_correct int NOT NULL DEFAULT 0,
  attempts     int NOT NULL DEFAULT 0,
  completed_at timestamptz,
  PRIMARY KEY (profile_id, variant_id)
);

CREATE TABLE question_memory (
  profile_id       uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  question_id      uuid NOT NULL REFERENCES question(id) ON DELETE CASCADE,
  stability        real NOT NULL DEFAULT 0,
  difficulty       real NOT NULL DEFAULT 0,
  due_at           timestamptz NOT NULL,
  last_reviewed_at timestamptz,
  reps             int NOT NULL DEFAULT 0,
  lapses           int NOT NULL DEFAULT 0,
  state            smallint NOT NULL DEFAULT 0,
  PRIMARY KEY (profile_id, question_id)
);
CREATE INDEX question_memory_due_idx ON question_memory(profile_id, due_at);

CREATE TABLE category_mastery (
  profile_id  uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  category_id uuid NOT NULL REFERENCES category(id) ON DELETE CASCADE,
  mastery     real NOT NULL DEFAULT 0,
  seen        int NOT NULL DEFAULT 0,
  correct     int NOT NULL DEFAULT 0,
  updated_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (profile_id, category_id)
);

CREATE TABLE saved_question (
  profile_id  uuid NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  question_id uuid NOT NULL REFERENCES question(id) ON DELETE CASCADE,
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (profile_id, question_id)
);

CREATE TABLE streak (
  profile_id       uuid PRIMARY KEY REFERENCES profile(id) ON DELETE CASCADE,
  current          int NOT NULL DEFAULT 0,
  best             int NOT NULL DEFAULT 0,
  last_active_date date,
  daily_goal       int NOT NULL DEFAULT 10,
  today_done       int NOT NULL DEFAULT 0
);
