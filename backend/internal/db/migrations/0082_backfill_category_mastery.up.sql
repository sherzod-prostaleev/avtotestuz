-- Backfill category_mastery rows that were dropped or undercounted when
-- 0069_42_topic_taxonomy deleted the 13 legacy categories (ON DELETE CASCADE).
-- Both question_memory (FSRS reps/lapses) and session_answer preserve the
-- complete answer history keyed by question_id, which points to the 42
-- canonical categories.
--
-- Strictly additive and idempotent: never lowers an existing seen/correct
-- counter and never deletes any row.
WITH qm_agg AS (
  SELECT qm.profile_id,
         q.category_id,
         sum(qm.reps)::int AS seen,
         sum(GREATEST(qm.reps - qm.lapses, 0))::int AS correct
  FROM question_memory qm
  JOIN question q ON q.id = qm.question_id
  WHERE qm.reps > 0
  GROUP BY qm.profile_id, q.category_id
), sa_agg AS (
  SELECT es.profile_id,
         q.category_id,
         count(*)::int AS seen,
         count(*) FILTER (WHERE sa.is_correct)::int AS correct
  FROM session_answer sa
  JOIN exam_session es ON es.id = sa.session_id
  JOIN question q ON q.id = sa.question_id
  GROUP BY es.profile_id, q.category_id
), combined AS (
  SELECT COALESCE(qm.profile_id, sa.profile_id) AS profile_id,
         COALESCE(qm.category_id, sa.category_id) AS category_id,
         GREATEST(COALESCE(qm.seen, 0), COALESCE(sa.seen, 0)) AS seen,
         LEAST(
           GREATEST(COALESCE(qm.correct, 0), COALESCE(sa.correct, 0)),
           GREATEST(COALESCE(qm.seen, 0), COALESCE(sa.seen, 0))
         ) AS correct
  FROM qm_agg qm
  FULL OUTER JOIN sa_agg sa USING (profile_id, category_id)
  WHERE GREATEST(COALESCE(qm.seen, 0), COALESCE(sa.seen, 0)) > 0
)
INSERT INTO category_mastery (profile_id, category_id, mastery, seen, correct, updated_at)
SELECT c.profile_id,
       c.category_id,
       (c.correct::real / c.seen::real) AS mastery,
       c.seen,
       c.correct,
       now()
FROM combined c
JOIN profile p ON p.id = c.profile_id
JOIN category cat ON cat.id = c.category_id
ON CONFLICT (profile_id, category_id) DO UPDATE SET
  seen       = GREATEST(category_mastery.seen, EXCLUDED.seen),
  correct    = LEAST(
                 GREATEST(category_mastery.correct, EXCLUDED.correct),
                 GREATEST(category_mastery.seen, EXCLUDED.seen)
               ),
  mastery    = LEAST(
                 GREATEST(category_mastery.correct, EXCLUDED.correct),
                 GREATEST(category_mastery.seen, EXCLUDED.seen)
               )::real / GREATEST(category_mastery.seen, EXCLUDED.seen, 1)::real,
  updated_at = now()
WHERE category_mastery.seen < EXCLUDED.seen
   OR category_mastery.correct < EXCLUDED.correct;
