-- DB-1: audit-trigger write amplification on content tables, plus a
-- retention path for admin_audit_log.
--
-- (a) Stop bulk-content churn. Migration 0051's AFTER INSERT/UPDATE/DELETE
-- fallback on question/answer/*_translation/explanation fires once per row,
-- so a single content re-import (1231 questions, 4015 answers, 3693
-- question_translations, 12045 answer_translations, 1219 explanations,
-- 3657 explanation_translations) writes 25,860 admin_audit_log rows — and a
-- re-import is an UPDATE, so every one of those rows carries both a before
-- and an after JSONB blob. admin_audit_log can never shrink on its own
-- (append-only — see admin_audit_log_immutable below), so this is
-- unbounded growth against content that already has its own audit trail.
--
-- These six tables are dropped from the fallback outright rather than made
-- conditional on a session flag, because a flag would have protected
-- nothing real: the trigger writes admin_user_id = NULL for every caller
-- (it has no notion of "who" — TG_ARGV only carries the table/key name), so
-- a bulk-import row and a real admin edit already look identical and
-- equally uninformative in admin_audit_log. Meanwhile genuine admin content
-- edits (patchQuestion, verifyQuestionExplanation, bulkVerifyExplanations
-- in internal/admin/handlers.go) write BOTH an attributed admin_audit_log
-- row via Store.WriteAudit (action "content.questions.patch" /
-- "content.verify" / "content.verify.bulk", real admin_user_id) AND an
-- immutable content_revision snapshot via Store.InsertContentRevision
-- (migration 0040, guarded by its own content_revision_immutable trigger,
-- carrying editor_id + note) — both richer and better-attributed than the
-- trigger's row, on the one write path where "an application-side audit
-- failure must still leave a trace" is a real concern. The trigger's only
-- distinct value was covering the bulk-importer path (internal/importer),
-- which never calls WriteAudit or InsertContentRevision at all — but that
-- path has no admin identity to protect in the first place; it is offline
-- content ingestion from a curated dataset file, not an attributable admin
-- mutation. The sensitive tables from 0051 (profile, entitlement, payment,
-- payment_provider_status, feature_flag, limit_config, site_settings,
-- manual_*, referral_*, b2b_*, support_ticket) have no equivalent second
-- app-level trail, so their triggers are untouched by this migration.
DROP TRIGGER IF EXISTS question_atomic_audit ON question;
DROP TRIGGER IF EXISTS question_translation_atomic_audit ON question_translation;
DROP TRIGGER IF EXISTS answer_atomic_audit ON answer;
DROP TRIGGER IF EXISTS answer_translation_atomic_audit ON answer_translation;
DROP TRIGGER IF EXISTS explanation_atomic_audit ON explanation;
DROP TRIGGER IF EXISTS explanation_translation_atomic_audit ON explanation_translation;
-- write_atomic_audit_fallback() itself is untouched: profile, entitlement,
-- payment, payment_provider_status, feature_flag, limit_config,
-- site_settings, support_ticket, b2b_*, referral_*, manual_* still use it.

-- (b) Give admin_audit_log a retention path without weakening its
-- append-only guarantee for live rows. admin_audit_log_immutable currently
-- rejects every UPDATE/DELETE unconditionally, which is correct for live
-- rows but leaves no legal way to ever shrink the table — the audit log
-- would grow forever even after content is safely re-attributed elsewhere.
-- Replace its trigger function with one that (1) keeps UPDATE fully blocked
-- at any age — audit rows are corrected by inserting a new row, never by
-- editing history — and (2) adds one narrowly-scoped DELETE exception: only
-- rows older than an operator-chosen retention window, and only while
-- archive_admin_audit_log() below is running (it sets a session-local GUC
-- for the lifetime of its own transaction, via set_config(..., true), so it
-- cannot leak into any other session or outlive its own commit/rollback). A
-- plain DELETE from application code, a client session, or any other caller
-- still hits the same wall it always did. content_revision_immutable is
-- untouched — it keeps using the original reject_immutable_row_change()
-- with no exception of any kind; content revisions are out of scope here.
CREATE OR REPLACE FUNCTION reject_immutable_audit_row_change()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  retention_days int;
BEGIN
  IF TG_OP = 'DELETE'
     AND current_setting('avtotest.audit_archive_in_progress', true) = 'on' THEN
    retention_days := NULLIF(current_setting('avtotest.audit_archive_retention_days', true), '')::int;
    IF retention_days IS NOT NULL AND retention_days > 0
       AND OLD.created_at < now() - make_interval(days => retention_days) THEN
      RETURN OLD;
    END IF;
  END IF;

  RAISE EXCEPTION '% is append-only (delete of a live or under-retention row is not permitted)', TG_TABLE_NAME
    USING ERRCODE = '55000';
END;
$$;

DROP TRIGGER IF EXISTS admin_audit_log_immutable ON admin_audit_log;
CREATE TRIGGER admin_audit_log_immutable
BEFORE UPDATE OR DELETE ON admin_audit_log
FOR EACH ROW EXECUTE FUNCTION reject_immutable_audit_row_change();

-- Cold storage for archived rows. Explicit columns (not "LIKE
-- admin_audit_log INCLUDING ALL") so this table's own indexing is chosen
-- for archive access patterns instead of inheriting the live table's.
CREATE TABLE admin_audit_log_archive (
  id            uuid PRIMARY KEY,
  admin_user_id uuid REFERENCES admin_user(id) ON DELETE SET NULL,
  action        text NOT NULL,
  entity_type   text NOT NULL,
  entity_id     text,
  before_json   jsonb,
  after_json    jsonb,
  ip            inet,
  ua            text NOT NULL DEFAULT '',
  request_id    text NOT NULL DEFAULT '',
  created_at    timestamptz NOT NULL,
  archived_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX admin_audit_log_archive_created_idx ON admin_audit_log_archive (created_at DESC);

-- Operator-invoked archival: copies admin_audit_log rows older than
-- retention_days into admin_audit_log_archive, then deletes them from the
-- live table through the narrowly-scoped trigger exception above. This
-- migration does not schedule it (no pg_cron dependency, no default
-- retention imposed) — an operator runs
-- `SELECT archive_admin_audit_log(730);` by hand, or wires it into a
-- maintenance job, when the live table needs pruning. The >= 365 floor
-- exists so a typo (e.g. days => 7) cannot sweep this week's audit trail;
-- widen it at the call site for a longer window, never lower it here.
CREATE OR REPLACE FUNCTION archive_admin_audit_log(retention_days int)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
  moved bigint;
BEGIN
  IF retention_days IS NULL OR retention_days < 365 THEN
    RAISE EXCEPTION 'retention_days must be >= 365 (got %)', retention_days;
  END IF;

  INSERT INTO admin_audit_log_archive
    (id, admin_user_id, action, entity_type, entity_id, before_json, after_json, ip, ua, request_id, created_at)
  SELECT id, admin_user_id, action, entity_type, entity_id, before_json, after_json, ip, ua, request_id, created_at
  FROM admin_audit_log
  WHERE created_at < now() - make_interval(days => retention_days)
  ON CONFLICT (id) DO NOTHING;

  PERFORM set_config('avtotest.audit_archive_in_progress', 'on', true);
  PERFORM set_config('avtotest.audit_archive_retention_days', retention_days::text, true);

  DELETE FROM admin_audit_log
  WHERE created_at < now() - make_interval(days => retention_days);
  GET DIAGNOSTICS moved = ROW_COUNT;

  PERFORM set_config('avtotest.audit_archive_in_progress', 'off', true);

  RETURN moved;
END;
$$;

COMMENT ON FUNCTION archive_admin_audit_log(int) IS
  'Retention path for admin_audit_log: copies rows older than retention_days into admin_audit_log_archive, then deletes them from the live table via the narrowly-scoped exception in reject_immutable_audit_row_change(). Operator-invoked; no default schedule, minimum retention_days is 365.';
