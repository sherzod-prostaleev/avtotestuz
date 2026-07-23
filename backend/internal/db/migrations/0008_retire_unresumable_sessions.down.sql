-- Intentionally irreversible: the legacy unanswered assignment was never
-- stored, so restoring these sessions to in_progress would make them appear
-- resumable while their question set is still incomplete.
SELECT 1;
