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

  it("does not replace a real local MinIO diagram with the cars placeholder", () => {
    const url = "http://localhost:9000/media/images/4567aa175f412cf4822198b6526e414cc38e34947a036e02fcc516bf82b81070.webp";
    expect(hasQuestionImage(url)).toBe(true);
    expect(resolveQuestionImageUrl(url)).not.toBe(QUESTION_IMAGE_PLACEHOLDER);
  });

  it("rewrites local MinIO URLs to same-origin /media so CSP img-src 'self' can load them", () => {
    const key = "images/4567aa175f412cf4822198b6526e414cc38e34947a036e02fcc516bf82b81070.webp";
    expect(resolveQuestionImageUrl(`http://localhost:9000/media/${key}`)).toBe(`/media/${key}`);
    expect(resolveQuestionImageUrl(`http://127.0.0.1:9000/media/${key}`)).toBe(`/media/${key}`);
    expect(resolveQuestionImageUrl(`http://minio:9000/media/${key}`)).toBe(`/media/${key}`);
  });

  it("leaves production and already-relative media URLs alone", () => {
    expect(resolveQuestionImageUrl("https://drivergo.uz/media/images/diagram.webp")).toBe(
      "https://drivergo.uz/media/images/diagram.webp"
    );
    expect(resolveQuestionImageUrl("/media/images/diagram.webp")).toBe("/media/images/diagram.webp");
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

  it("rewrites upcoming local MinIO URLs to same-origin /media for prefetch", () => {
    expect(
      upcomingQuestionImageUrls(
        [
          { image_url: "http://localhost:9000/media/images/a.webp" },
          { image_url: "http://127.0.0.1:9000/media/images/b.webp" },
        ],
        0
      )
    ).toEqual(["/media/images/a.webp", "/media/images/b.webp"]);
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
