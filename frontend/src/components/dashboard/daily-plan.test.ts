import { describe, expect, it } from "vitest";
import { buildDailyPlan, hasSatExamToday } from "./daily-plan";
import type { SessionSummary } from "@/hooks/use-session-history";

const NOW = new Date("2026-09-01T14:00:00");

function session(over: Partial<SessionSummary>): SessionSummary {
  return {
    id: "s1",
    mode: "exam",
    status: "passed",
    total: 20,
    started_at: "2026-09-01T10:00:00Z",
    finished_at: "2026-09-01T10:20:00",
    ...over,
  };
}

const BASE = { todayAnswered: 0, dailyTarget: 10, dueCount: 0, sessions: [], now: NOW };

describe("buildDailyPlan", () => {
  it("puts the button on the first unfinished step", () => {
    const plan = buildDailyPlan(BASE);
    expect(plan.activeKey).toBe("daily");
    expect(plan.steps.filter((step) => step.active)).toHaveLength(1);
  });

  it("moves the button on once the daily target is met", () => {
    const plan = buildDailyPlan({ ...BASE, todayAnswered: 10, dueCount: 4 });
    expect(plan.activeKey).toBe("review");
    expect(plan.doneCount).toBe(1);
  });

  // A learner with no due questions has genuinely finished that step; it must
  // not swallow the button and leave the screen without a call to action.
  it("skips an empty review queue", () => {
    const plan = buildDailyPlan({ ...BASE, todayAnswered: 10, dueCount: 0 });
    expect(plan.activeKey).toBe("exam");
    expect(plan.doneCount).toBe(2);
  });

  it("reports the day finished once an exam has been sat", () => {
    const plan = buildDailyPlan({
      ...BASE,
      todayAnswered: 10,
      dueCount: 0,
      sessions: [session({})],
    });
    expect(plan.activeKey).toBeNull();
    expect(plan.doneCount).toBe(3);
  });

  // A zero target arrives whenever `me/streak` has not answered yet; marking
  // the step done then would show "1/3" to a learner who has done nothing.
  it("never counts a zero daily target as met", () => {
    const plan = buildDailyPlan({ ...BASE, dailyTarget: 0, todayAnswered: 0 });
    expect(plan.steps[0].done).toBe(false);
    expect(plan.activeKey).toBe("daily");
  });

  it("keeps the three steps in a fixed order", () => {
    expect(buildDailyPlan(BASE).steps.map((step) => step.key)).toEqual(["daily", "review", "exam"]);
  });
});

describe("hasSatExamToday", () => {
  it("counts a failed exam — the step is sitting one, not passing one", () => {
    expect(hasSatExamToday([session({ status: "failed" })], NOW)).toBe(true);
  });

  it("counts the grand mock", () => {
    expect(hasSatExamToday([session({ mode: "grand_mock" })], NOW)).toBe(true);
  });

  it("ignores practice, abandoned and still-running sessions", () => {
    expect(hasSatExamToday([session({ mode: "practice" })], NOW)).toBe(false);
    expect(hasSatExamToday([session({ status: "abandoned" })], NOW)).toBe(false);
    expect(hasSatExamToday([session({ status: "in_progress", finished_at: undefined })], NOW)).toBe(false);
  });

  it("ignores an exam sat on another day", () => {
    expect(hasSatExamToday([session({ finished_at: "2026-08-31T22:00:00" })], NOW)).toBe(false);
  });

  it("survives a malformed timestamp instead of throwing", () => {
    expect(hasSatExamToday([session({ finished_at: "not-a-date" })], NOW)).toBe(false);
  });
});
