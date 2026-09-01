import type { SessionSummary } from "@/hooks/use-session-history";

/**
 * The three things a learner is asked to do on a given day, in order.
 *
 * The phone dashboard shows exactly one call to action — the first step that
 * is not done yet — because the old screen offered eleven blocks and three
 * identical "start practice" buttons and answered nobody's "what do I do now".
 * Every input here already exists on the dashboard (`me/streak`, `me/stats`,
 * `me/sessions`); nothing new is fetched for the plan.
 */
export type DailyPlanStepKey = "daily" | "review" | "exam";

export interface DailyPlanStep {
  key: DailyPlanStepKey;
  done: boolean;
  /** The one step carrying the primary button: the first that is not done. */
  active: boolean;
}

export interface DailyPlanInput {
  todayAnswered: number;
  dailyTarget: number;
  dueCount: number;
  sessions: SessionSummary[];
  /** Injected in tests; the day boundary is the viewer's local midnight. */
  now?: Date;
}

export interface DailyPlan {
  steps: DailyPlanStep[];
  doneCount: number;
  /** Null once all three are done — the card then shows its finished state. */
  activeKey: DailyPlanStepKey | null;
}

function isSameLocalDay(iso: string | undefined, now: Date): boolean {
  if (!iso) return false;
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return false;
  return (
    at.getFullYear() === now.getFullYear() &&
    at.getMonth() === now.getMonth() &&
    at.getDate() === now.getDate()
  );
}

/**
 * A sat exam counts whether it was passed or failed — the step is "you tried
 * one today", not "you passed one" — and an abandoned or still-running session
 * does not count.
 */
export function hasSatExamToday(sessions: SessionSummary[], now: Date): boolean {
  return sessions.some(
    (session) =>
      (session.mode === "exam" || session.mode === "grand_mock") &&
      (session.status === "passed" || session.status === "failed") &&
      isSameLocalDay(session.finished_at ?? session.started_at, now)
  );
}

export function buildDailyPlan({
  todayAnswered,
  dailyTarget,
  dueCount,
  sessions,
  now = new Date(),
}: DailyPlanInput): DailyPlan {
  // A target of zero would otherwise mark the step done before any work.
  const dailyDone = dailyTarget > 0 && todayAnswered >= dailyTarget;
  const done: Record<DailyPlanStepKey, boolean> = {
    daily: dailyDone,
    review: dueCount <= 0,
    exam: hasSatExamToday(sessions, now),
  };

  const order: DailyPlanStepKey[] = ["daily", "review", "exam"];
  const activeKey = order.find((key) => !done[key]) ?? null;

  return {
    steps: order.map((key) => ({ key, done: done[key], active: key === activeKey })),
    doneCount: order.filter((key) => done[key]).length,
    activeKey,
  };
}
