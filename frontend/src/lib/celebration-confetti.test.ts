import { describe, expect, it } from "vitest";
import { EXAM_ERRORS_ALLOWED, meetsExamPassThreshold } from "./celebration-confetti";

describe("meetsExamPassThreshold", () => {
  it("matches backend exam bar: 18/20 passes, 17/20 fails", () => {
    expect(EXAM_ERRORS_ALLOWED).toBe(2);
    expect(meetsExamPassThreshold(20, 20)).toBe(true);
    expect(meetsExamPassThreshold(18, 20)).toBe(true);
    expect(meetsExamPassThreshold(17, 20)).toBe(false);
    expect(meetsExamPassThreshold(0, 20)).toBe(false);
  });

  it("rejects empty totals", () => {
    expect(meetsExamPassThreshold(0, 0)).toBe(false);
  });
});
