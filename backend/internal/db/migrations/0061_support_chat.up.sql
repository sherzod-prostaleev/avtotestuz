-- Real-time support chat (Telegram-style thread per learner).
-- Keeps legacy support_ticket rows intact for historical data safety.
-- Does NOT add password_plain — password recovery uses must_change_password (0060).

CREATE TABLE support_conversation (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id      uuid NOT NULL UNIQUE REFERENCES profile(id) ON DELETE CASCADE,
  status          text NOT NULL DEFAULT 'open'
                  CHECK (status IN ('open', 'closed')),
  -- Unread counters: incremented for the other party on insert; cleared on read.
  unread_admin    int NOT NULL DEFAULT 0 CHECK (unread_admin >= 0),
  unread_user     int NOT NULL DEFAULT 0 CHECK (unread_user >= 0),
  last_message_at timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE support_message (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  conversation_id   uuid NOT NULL REFERENCES support_conversation(id) ON DELETE CASCADE,
  sender_kind       text NOT NULL CHECK (sender_kind IN ('user', 'admin')),
  sender_profile_id uuid REFERENCES profile(id) ON DELETE SET NULL,
  sender_admin_id   uuid REFERENCES admin_user(id) ON DELETE SET NULL,
  body              text NOT NULL DEFAULT '',
  attachment_key    text NOT NULL DEFAULT '',
  attachment_name   text NOT NULL DEFAULT '',
  attachment_mime   text NOT NULL DEFAULT '',
  attachment_size   bigint NOT NULL DEFAULT 0,
  created_at        timestamptz NOT NULL DEFAULT now(),
  CHECK (body <> '' OR attachment_key <> '')
);

CREATE INDEX support_message_conv_created_idx
  ON support_message (conversation_id, created_at);
CREATE INDEX support_conversation_last_msg_idx
  ON support_conversation (last_message_at DESC NULLS LAST);

CREATE TRIGGER support_conversation_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON support_conversation
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('support_conversation', 'id');

CREATE TRIGGER support_message_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON support_message
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('support_message', 'id');
