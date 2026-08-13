import { describe, expect, it, vi } from "vitest";
import {
  QUESTION_IMAGE_PLACEHOLDER,
  hasQuestionImage,
  prefetchQuestionImages,
  resolveQuestionImageUrl,
  upcomingQuestionImageUrls,
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

  it("collects the current and next two real media URLs", () => {
    expect(
      upcomingQuestionImageUrls(
        [
          { image_url: "https://media.example.test/a.webp" },
          { image_url: null },
          { image_url: "https://media.example.test/c.webp" },
          { image_url: "https://media.example.test/d.webp" },
        ],
        0
      )
    ).toEqual(["https://media.example.test/a.webp", "https://media.example.test/c.webp"]);
  });

  it("prefetches unique real URLs and skips the placeholder", () => {
    const sources: string[] = [];
    class FakeImage {
      set src(value: string) {
        sources.push(value);
      }
    }
    vi.stubGlobal("Image", FakeImage);
    prefetchQuestionImages([
      "https://media.example.test/a.webp",
      null,
      "https://media.example.test/a.webp",
      QUESTION_IMAGE_PLACEHOLDER,
      "  https://media.example.test/b.webp  ",
    ]);
    expect(sources).toEqual([
      "https://media.example.test/a.webp",
      "https://media.example.test/b.webp",
    ]);
    vi.unstubAllGlobals();
  });
});
