import { describe, expect, it } from "vitest";
import {
  QUESTION_IMAGE_PLACEHOLDER,
  hasQuestionImage,
  resolveQuestionImageUrl,
} from "./question-image";

describe("question-image", () => {
  it("treats null, empty, and whitespace as missing", () => {
    expect(hasQuestionImage(null)).toBe(false);
    expect(hasQuestionImage(undefined)).toBe(false);
    expect(hasQuestionImage("")).toBe(false);
    expect(hasQuestionImage("   ")).toBe(false);
  });

  it("keeps a real media URL unchanged", () => {
    const url = "https://media.example.test/diagram.webp";
    expect(hasQuestionImage(url)).toBe(true);
    expect(resolveQuestionImageUrl(url)).toBe(url);
  });

  it("falls back to the Driver Go cars placeholder when missing", () => {
    expect(resolveQuestionImageUrl(null)).toBe(QUESTION_IMAGE_PLACEHOLDER);
    expect(resolveQuestionImageUrl("")).toBe(QUESTION_IMAGE_PLACEHOLDER);
    expect(resolveQuestionImageUrl("  ")).toBe(QUESTION_IMAGE_PLACEHOLDER);
  });
});
