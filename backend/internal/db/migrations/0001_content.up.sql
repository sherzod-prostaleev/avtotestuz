CREATE DOMAIN locale_code AS text
  CHECK (VALUE IN ('uz-Latn','uz-Cyrl','ru','kaa'));

CREATE TABLE category (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code        text NOT NULL UNIQUE,
  sort_order  int  NOT NULL DEFAULT 0,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE category_translation (
  category_id uuid NOT NULL REFERENCES category(id) ON DELETE CASCADE,
  locale      locale_code NOT NULL,
  name        text NOT NULL,
  status      text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','verified')),
  PRIMARY KEY (category_id, locale)
);

CREATE TABLE image (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  storage_key text NOT NULL UNIQUE,
  sha256      text NOT NULL UNIQUE,
  width       int  NOT NULL DEFAULT 0,
  height      int  NOT NULL DEFAULT 0,
  mime        text NOT NULL,
  source      text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE question (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_ext_id     text NOT NULL UNIQUE,
  category_id       uuid NOT NULL REFERENCES category(id),
  image_id          uuid REFERENCES image(id),
  correct_answer_id uuid,  -- set after answers exist (same tx); FK added below
  content_hash      text NOT NULL,
  source            text NOT NULL DEFAULT '',
  validation_status text NOT NULL DEFAULT 'valid'
                    CHECK (validation_status IN ('valid','quarantined')),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX question_category_idx ON question(category_id) WHERE validation_status = 'valid';

CREATE TABLE answer (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  question_id uuid NOT NULL REFERENCES question(id) ON DELETE CASCADE,
  position    smallint NOT NULL CHECK (position BETWEEN 1 AND 4),
  is_correct  boolean NOT NULL DEFAULT false,
  image_id    uuid REFERENCES image(id),
  UNIQUE (question_id, position)
);

ALTER TABLE question
  ADD CONSTRAINT question_correct_answer_fk
  FOREIGN KEY (correct_answer_id) REFERENCES answer(id);

CREATE TABLE question_translation (
  question_id uuid NOT NULL REFERENCES question(id) ON DELETE CASCADE,
  locale      locale_code NOT NULL,
  text        text NOT NULL,
  status      text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','verified')),
  source      text NOT NULL DEFAULT '',
  PRIMARY KEY (question_id, locale)
);

CREATE TABLE answer_translation (
  answer_id uuid NOT NULL REFERENCES answer(id) ON DELETE CASCADE,
  locale    locale_code NOT NULL,
  text      text NOT NULL,
  status    text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','verified')),
  PRIMARY KEY (answer_id, locale)
);

CREATE TABLE variant (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  number     int NOT NULL UNIQUE CHECK (number > 0),
  sort_order int NOT NULL DEFAULT 0
);

CREATE TABLE variant_question (
  variant_id  uuid NOT NULL REFERENCES variant(id) ON DELETE CASCADE,
  question_id uuid NOT NULL REFERENCES question(id) ON DELETE CASCADE,
  position    smallint NOT NULL CHECK (position >= 1),
  PRIMARY KEY (variant_id, position),
  UNIQUE (variant_id, question_id)
);

CREATE TABLE sign_group (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code       text NOT NULL UNIQUE,
  sort_order int NOT NULL DEFAULT 0
);

CREATE TABLE sign_group_translation (
  sign_group_id uuid NOT NULL REFERENCES sign_group(id) ON DELETE CASCADE,
  locale        locale_code NOT NULL,
  name          text NOT NULL,
  status        text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','verified')),
  PRIMARY KEY (sign_group_id, locale)
);

CREATE TABLE sign (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  group_id   uuid NOT NULL REFERENCES sign_group(id),
  code       text NOT NULL UNIQUE,
  image_id   uuid REFERENCES image(id),
  sort_order int NOT NULL DEFAULT 0
);

CREATE TABLE sign_translation (
  sign_id     uuid NOT NULL REFERENCES sign(id) ON DELETE CASCADE,
  locale      locale_code NOT NULL,
  name        text NOT NULL,
  description text NOT NULL DEFAULT '',
  status      text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','verified')),
  PRIMARY KEY (sign_id, locale)
);

CREATE TABLE question_sign (
  question_id uuid NOT NULL REFERENCES question(id) ON DELETE CASCADE,
  sign_id     uuid NOT NULL REFERENCES sign(id) ON DELETE CASCADE,
  PRIMARY KEY (question_id, sign_id)
);
CREATE INDEX question_sign_sign_idx ON question_sign(sign_id);

CREATE TABLE explanation (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  question_id uuid NOT NULL UNIQUE REFERENCES question(id) ON DELETE CASCADE,
  legal_refs  jsonb NOT NULL DEFAULT '[]',
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE explanation_translation (
  explanation_id uuid NOT NULL REFERENCES explanation(id) ON DELETE CASCADE,
  locale         locale_code NOT NULL,
  blocks         jsonb NOT NULL DEFAULT '[]',
  status         text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','pending','verified')),
  verified_by    uuid,
  verified_at    timestamptz,
  source         text NOT NULL DEFAULT '',
  PRIMARY KEY (explanation_id, locale)
);
