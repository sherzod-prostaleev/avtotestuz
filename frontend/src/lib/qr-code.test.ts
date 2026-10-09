import { describe, expect, it } from "vitest";
import { encodeQr, qrSvgPath } from "./qr-code";

// Decodability was checked with zbarimg against these exact inputs (a login
// deep link, a short text and a version-7+ payload with version info); the
// unit tests pin the structure so a refactor cannot silently break it.
describe("encodeQr", () => {
  const link = `https://t.me/DriverGouzBot?start=login_${"A".repeat(43)}`;

  it("picks the smallest version for level M", () => {
    expect(encodeQr("hello")).toHaveLength(21); // version 1
    expect(encodeQr(link)).toHaveLength(37); // 82 bytes -> version 5
    expect(encodeQr("x".repeat(150))).toHaveLength(49); // version 8
  });

  it("draws the three finder patterns and the dark module", () => {
    const m = encodeQr(link);
    const size = m.length;
    for (const [x, y] of [
      [0, 0],
      [size - 7, 0],
      [0, size - 7],
    ]) {
      // Outer ring dark, ring inside it light, 3x3 core dark.
      expect(m[y][x]).toBe(true);
      expect(m[y + 1][x + 1]).toBe(false);
      expect(m[y + 3][x + 3]).toBe(true);
    }
    expect(m[size - 8][8]).toBe(true);
  });

  it("is deterministic and refuses what does not fit", () => {
    expect(encodeQr(link)).toEqual(encodeQr(link));
    expect(() => encodeQr("x".repeat(300))).toThrow();
  });

  it("renders an SVG path with a quiet zone", () => {
    const { path, size } = qrSvgPath(encodeQr("hello"));
    expect(size).toBe(29);
    expect(path.startsWith("M4 4h1v1h-1z")).toBe(true);
  });
});
