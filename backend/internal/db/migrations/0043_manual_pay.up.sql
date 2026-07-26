-- Manual Humo card P2P checkout + Humocardbot push matching.

-- Widen provider CHECKs for payment + kill-switch table.
ALTER TABLE payment DROP CONSTRAINT IF EXISTS payment_provider_check;
ALTER TABLE payment ADD CONSTRAINT payment_provider_check
  CHECK (provider IN ('payme', 'click', 'sandbox', 'manual'));

ALTER TABLE payment_provider_status DROP CONSTRAINT IF EXISTS payment_provider_status_provider_check;
ALTER TABLE payment_provider_status ADD CONSTRAINT payment_provider_status_provider_check
  CHECK (provider IN ('payme', 'click', 'manual'));

INSERT INTO payment_provider_status (provider, enabled) VALUES ('manual', true)
ON CONFLICT (provider) DO NOTHING;

-- Learner-visible checkout gate (AND with kill-switch).
INSERT INTO feature_flag (key, type, value_json, description) VALUES
  ('checkout_manual', 'boolean', 'true'::jsonb, 'Show manual Humo card transfer on checkout')
ON CONFLICT (key) DO NOTHING;

-- Receiving cards (admin-managed pool; typically 4 Humo cards).
CREATE TABLE manual_pay_card (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  network text NOT NULL DEFAULT 'humo' CHECK (network IN ('humo')),
  pan_full text NOT NULL,
  pan_last4 text NOT NULL CHECK (pan_last4 ~ '^[0-9]{4}$'),
  holder_name text NOT NULL,
  sort_order int NOT NULL DEFAULT 0,
  enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (pan_last4)
);

-- One assignment per payment: card hold + lifecycle for Path B (late push OK).
CREATE TABLE manual_pay_assignment (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  payment_id uuid NOT NULL UNIQUE REFERENCES payment(id),
  card_id uuid NOT NULL REFERENCES manual_pay_card(id),
  amount_uzs bigint NOT NULL CHECK (amount_uzs > 0),
  assigned_at timestamptz NOT NULL DEFAULT now(),
  hold_until timestamptz NOT NULL,
  released_at timestamptz,
  -- awaiting_transfer → claimed → awaiting_review → consumed | rejected
  manual_state text NOT NULL DEFAULT 'awaiting_transfer'
    CHECK (manual_state IN ('awaiting_transfer', 'claimed', 'awaiting_review', 'consumed', 'rejected')),
  claimed_at timestamptz,
  confirmed_by text, -- bot | admin | NULL
  confirmed_at timestamptz,
  admin_note text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX manual_pay_assignment_open_idx
  ON manual_pay_assignment (card_id, amount_uzs, assigned_at)
  WHERE manual_state IN ('awaiting_transfer', 'claimed', 'awaiting_review');

CREATE INDEX manual_pay_assignment_hold_idx
  ON manual_pay_assignment (hold_until)
  WHERE released_at IS NULL AND manual_state IN ('awaiting_transfer', 'claimed');

-- Parsed Humocardbot pushes (idempotent by fingerprint).
CREATE TABLE manual_pay_event (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  fingerprint text NOT NULL UNIQUE,
  telegram_msg_id bigint,
  raw_text text NOT NULL,
  amount_uzs bigint,
  pan_last4 text,
  transfer_at timestamptz,
  balance_uzs bigint,
  status text NOT NULL DEFAULT 'unmatched'
    CHECK (status IN ('matched', 'unmatched', 'ignored')),
  matched_payment_id uuid REFERENCES payment(id),
  parse_ok boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX manual_pay_event_unmatched_idx
  ON manual_pay_event (created_at DESC)
  WHERE status = 'unmatched';

-- Telethon userbot credentials (AES-GCM ciphertext, admin-managed).
CREATE TABLE manual_tg_settings (
  id int PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  api_id_enc text,
  api_hash_enc text,
  session_enc text,
  humo_bot_username text NOT NULL DEFAULT 'HUMOcardbot',
  last_test_ok boolean,
  last_test_at timestamptz,
  last_test_detail text,
  updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO manual_tg_settings (id) VALUES (1);
