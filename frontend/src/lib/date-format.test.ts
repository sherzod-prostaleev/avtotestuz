import { describe, expect, it } from "vitest";
import { formatDateShort, formatDateWithTime } from "./date-format";

describe("date-format utility", () => {
  it("formats short date as DD.MM.YY", () => {
    const d = new Date(2026, 6, 24, 22, 38); // 24 July 2026
    expect(formatDateShort(d)).toBe("24.07.26");
  });

  it("formats date with time as DD.MM.YY HH:mm", () => {
    const d = new Date(2026, 6, 24, 22, 38);
    expect(formatDateWithTime(d)).toBe("24.07.26 22:38");
  });

  it("handles string dates", () => {
    expect(formatDateShort("2026-07-24T22:38:00Z")).not.toBe("");
  });

  it("handles invalid inputs gracefully", () => {
    expect(formatDateShort(null)).toBe("");
    expect(formatDateShort(undefined)).toBe("");
    expect(formatDateWithTime("invalid")).toBe("invalid");
  });
});
