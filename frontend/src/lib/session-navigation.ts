import type { SessionQuestionItem } from "@/hooks/use-session-engine";

/**
 * How long a graded answer stays on screen before the runner hops to the next
 * question. Long enough to register the green/red state, short enough that a
 * 50-question restore exam does not feel like 50 pauses.
 */
export const AUTO_ADVANCE_MS = 900;

/**
 * Whether this question already carries the learner's answer. `answered` is
 * the server's flag; `user_answer_id` covers the window where an optimistic
 * local update has recorded the choice but the flag has not been refreshed.
 */
export function hasAnswer(question: SessionQuestionItem): boolean {
  return question.answered === true || Boolean(question.user_answer_id);
}

/**
 * Index of the next question still waiting for an answer: forward from `from`
 * first, then wrapping to the start so a learner who jumped around is carried
 * back to the gaps they left. Returns -1 when nothing is left, which is the
 * caller's signal to stay put and let the auto-finish effect close the session.
 *
 * `justAnsweredId` is counted as answered even if the questions array has not
 * been refreshed yet — the hop is scheduled immediately after a submit.
 */
export function nextUnansweredIndex(
  questions: SessionQuestionItem[],
  from: number,
  justAnsweredId?: string
): number {
  const answered = (question: SessionQuestionItem) =>
    hasAnswer(question) || question.id === justAnsweredId;

  for (let i = from + 1; i < questions.length; i++) {
    if (!answered(questions[i])) return i;
  }
  for (let i = 0; i < Math.min(from + 1, questions.length); i++) {
    if (!answered(questions[i])) return i;
  }
  return -1;
}
