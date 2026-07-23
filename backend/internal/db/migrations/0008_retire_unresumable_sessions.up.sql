-- Migration 0007 could only recover questions that had already been answered;
-- the unanswered randomized set never existed in the legacy schema. Such an
-- in-progress session cannot be resumed honestly. Retire every incomplete
-- assignment instead of returning a misleading total with an empty/partial
-- question list to clients.
UPDATE exam_session AS es
SET status = 'abandoned',
    finished_at = COALESCE(es.finished_at, now()),
    stopped_reason = NULL
WHERE es.status = 'in_progress'
  AND (
    SELECT count(*)
    FROM session_question AS sq
    WHERE sq.session_id = es.id
  ) <> es.total;
