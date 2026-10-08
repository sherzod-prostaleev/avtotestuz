import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { cssColorToHex } from "@/lib/telegram/color";

// The dark palette's --danger-ink is the Telegram unlink action's text on the
// website card; at 4.33:1 it failed WCAG AA for normal-size text.
const css = readFileSync(path.resolve(import.meta.dirname, "globals.css"), "utf8");

function darkToken(name: string): string {
  const block = css.slice(css.indexOf(":root,"), css.indexOf(".light {"));
  const m = block.match(new RegExp(`--${name}:\\s*([^;]+);`));
  if (!m) throw new Error(`--${name} not found in the dark block`);
  return m[1];
}

function luminance(hex: string): number {
  const ch = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255);
  const [r, g, b] = ch.map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

describe("dark palette ink contrast", () => {
  it.each(["card", "background"])("--danger-ink on --%s reaches 4.5:1", (surface) => {
    const ink = cssColorToHex(darkToken("danger-ink"))!;
    const bg = cssColorToHex(darkToken(surface))!;
    expect(contrast(ink, bg)).toBeGreaterThanOrEqual(4.5);
  });
});
