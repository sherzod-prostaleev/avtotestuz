-- Audit rows and content revisions are append-only. A database-level fallback
-- records sensitive admin-managed table mutations in the same transaction as
-- the mutation, so an application-side audit failure cannot leave no trace.

CREATE OR REPLACE FUNCTION reject_immutable_row_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = '55000';
END;
$$;

CREATE TRIGGER admin_audit_log_immutable
BEFORE UPDATE OR DELETE ON admin_audit_log
FOR EACH ROW EXECUTE FUNCTION reject_immutable_row_change();

CREATE TRIGGER content_revision_immutable
BEFORE UPDATE OR DELETE ON content_revision
FOR EACH ROW EXECUTE FUNCTION reject_immutable_row_change();

CREATE OR REPLACE FUNCTION write_atomic_audit_fallback()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  before_row jsonb;
  after_row jsonb;
  entity_row jsonb;
  entity_key text;
BEGIN
  IF TG_OP = 'INSERT' THEN
    before_row := NULL;
    after_row := to_jsonb(NEW);
    entity_row := after_row;
  ELSIF TG_OP = 'UPDATE' THEN
    before_row := to_jsonb(OLD);
    after_row := to_jsonb(NEW);
    entity_row := after_row;
  ELSE
    before_row := to_jsonb(OLD);
    after_row := NULL;
    entity_row := before_row;
  END IF;

  -- Never duplicate credentials, password hashes, token material or PANs in
  -- the audit table. PAN columns are encrypted separately, but are still
  -- excluded here as defense in depth during rolling migrations.
  before_row := before_row - ARRAY[
    'password_hash', 'totp_secret_enc', 'refresh_hash', 'token_hash',
    'pan_full', 'card_number', 'telegram_session_enc', 'secret_enc',
    'api_id_enc', 'api_hash_enc', 'session_enc'
  ];
  after_row := after_row - ARRAY[
    'password_hash', 'totp_secret_enc', 'refresh_hash', 'token_hash',
    'pan_full', 'card_number', 'telegram_session_enc', 'secret_enc',
    'api_id_enc', 'api_hash_enc', 'session_enc'
  ];

  entity_key := COALESCE(entity_row ->> TG_ARGV[1], '');
  INSERT INTO admin_audit_log
    (admin_user_id, action, entity_type, entity_id, before_json, after_json, ua, request_id)
  VALUES
    (NULL, 'db.' || lower(TG_OP), TG_ARGV[0], NULLIF(entity_key, ''),
     before_row, after_row, 'database-trigger', '');

  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER profile_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON profile
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('profile', 'id');

CREATE TRIGGER entitlement_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON entitlement
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('entitlement', 'id');

CREATE TRIGGER question_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON question
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('question', 'id');

CREATE TRIGGER question_translation_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON question_translation
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('question_translation', 'question_id');

CREATE TRIGGER answer_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON answer
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('answer', 'id');

CREATE TRIGGER answer_translation_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON answer_translation
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('answer_translation', 'answer_id');

CREATE TRIGGER explanation_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON explanation
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('explanation', 'id');

CREATE TRIGGER explanation_translation_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON explanation_translation
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('explanation_translation', 'explanation_id');

CREATE TRIGGER payment_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON payment
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('payment', 'id');

CREATE TRIGGER payment_provider_status_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON payment_provider_status
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('payment_provider', 'provider');

CREATE TRIGGER feature_flag_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON feature_flag
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('feature_flag', 'key');

CREATE TRIGGER limit_config_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON limit_config
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('limit_config', 'key');

CREATE TRIGGER site_settings_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON site_settings
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('site_settings', 'key');

CREATE TRIGGER support_ticket_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON support_ticket
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('support_ticket', 'id');

CREATE TRIGGER b2b_org_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON b2b_org
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('b2b_org', 'id');

CREATE TRIGGER b2b_org_member_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON b2b_org_member
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('b2b_org_member', 'org_id');

CREATE TRIGGER b2b_org_license_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON b2b_org_license
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('b2b_org_license', 'id');

CREATE TRIGGER b2b_invite_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON b2b_invite
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('b2b_invite', 'id');

CREATE TRIGGER referral_payout_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON referral_payout
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('referral_payout', 'id');

CREATE TRIGGER referral_ledger_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON referral_ledger
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('referral_ledger', 'id');

CREATE TRIGGER manual_tg_settings_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON manual_tg_settings
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('manual_tg_settings', 'id');

CREATE TRIGGER manual_pay_card_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON manual_pay_card
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('manual_pay_card', 'id');

CREATE TRIGGER manual_pay_assignment_atomic_audit
AFTER INSERT OR UPDATE OR DELETE ON manual_pay_assignment
FOR EACH ROW EXECUTE FUNCTION write_atomic_audit_fallback('manual_pay_assignment', 'id');
