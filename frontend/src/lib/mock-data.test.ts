import { describe, it, expect } from "vitest";
import { demoQuestion, mockExamQuestions, mockProfile, mockCategoryMastery, proofStats } from "./mock-data";

describe("mock-data invariants", () => {
  it("every mock question's correctAnswerId matches one of its own answers", () => {
    for (const q of [demoQuestion, ...mockExamQuestions]) {
      const ids = q.answers.map((a) => a.id);
      expect(ids).toContain(q.correctAnswerId);
    }
  });

  it("every mock question has between 2 and 5 answers (matches the real content invariant)", () => {
    for (const q of [demoQuestion, ...mockExamQuestions]) {
      expect(q.answers.length).toBeGreaterThanOrEqual(2);
      expect(q.answers.length).toBeLessThanOrEqual(5);
    }
  });

  it("streak progress never exceeds the daily goal in the mock profile", () => {
    expect(mockProfile.streak.todayDone).toBeLessThanOrEqual(mockProfile.streak.dailyGoal);
  });

  it("category mastery percentages are within 0-100", () => {
    for (const c of mockCategoryMastery) {
      expect(c.masteryPercent).toBeGreaterThanOrEqual(0);
      expect(c.masteryPercent).toBeLessThanOrEqual(100);
    }
  });

  it("proof stats has exactly 4 entries matching the landing page design", () => {
    expect(proofStats).toHaveLength(4);
  });
});
