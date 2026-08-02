-- Refuse rollback while archived rows exist: once live rows are pruned by
-- archive_admin_audit_log(), admin_audit_log_archive is the only surviving
-- copy of that audit history, and dropping it would destroy it. Operators
-- must fix-forward instead (same pattern as 0047's payment_void guard).
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM admin_audit_log_archive) THEN
    RAISE EXCEPTION 'cannot roll back 0053: admin_audit_log_archive contains archived records';
  END IF;
END $$;

DROP FUNCTION IF EXISTS archive_admin_audit_log(int);
DROP TABLE IF EXISTS admin_audit_log_archive;

DROP TRIGGER IF EXISTS admin_audit_log_immutable ON admin_audit_log;
CREATE TRIGGER admin_audit_log_immutable
BEFORE UPDATE OR DELETE ON admin_audit_log
FOR EACH ROW EXECUTE FUNCTION reject_immutable_row_change();
DROP FUNCTION IF EXISTS reject_immutable_audit_row_change();

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
