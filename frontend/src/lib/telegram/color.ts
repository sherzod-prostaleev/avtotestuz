/**
 * Telegram's setHeaderColor/setBackgroundColor take only "#rrggbb". Our tokens
 * are shadcn-style bare HSL triplets ("220 22% 7%" in globals.css), but this
 * also accepts hsl()/rgb()/hex so a future token format does not silently
 * paint Telegram's frame black. Anything it cannot read is null, never a guess.
 */
export function cssColorToHex(raw: string): string | null {
  const value = raw.trim().toLowerCase();
  if (!value) return null;

  const hex = /^#([0-9a-f]{3}|[0-9a-f]{6})$/.exec(value);
  if (hex) {
    const digits = hex[1].length === 3 ? [...hex[1]].map((c) => c + c).join("") : hex[1];
    return `#${digits}`;
  }

  const fn = /^(hsla?|rgba?)\((.*)\)$/.exec(value);
  const kind = fn ? (fn[1].startsWith("hsl") ? "hsl" : "rgb") : "hsl";
  const body = fn ? fn[2] : value;
  // Drop an alpha part ("/ 0.5" or a fourth comma value); Telegram has no alpha.
  const parts = body.split("/")[0].split(/[\s,]+/).filter(Boolean).slice(0, 3);
  if (parts.length !== 3) return null;

  if (kind === "rgb") {
    const rgb = parts.map((p) => (p.endsWith("%") ? (Number.parseFloat(p) * 255) / 100 : Number(p)));
    return rgb.every(Number.isFinite) ? toHex(rgb[0], rgb[1], rgb[2]) : null;
  }

  const h = Number.parseFloat(parts[0].replace(/deg$/, ""));
  if (!parts[1].endsWith("%") || !parts[2].endsWith("%")) return null;
  const s = Number.parseFloat(parts[1]) / 100;
  const l = Number.parseFloat(parts[2]) / 100;
  if (![h, s, l].every(Number.isFinite) || !/^-?[\d.]+(deg)?$/.test(parts[0])) return null;
  return hslToHex(h, s, l);
}

function hslToHex(h: number, s: number, l: number): string {
  const hue = (((h % 360) + 360) % 360) / 60;
  const chroma = (1 - Math.abs(2 * l - 1)) * s;
  const x = chroma * (1 - Math.abs((hue % 2) - 1));
  const [r, g, b] =
    hue < 1 ? [chroma, x, 0]
    : hue < 2 ? [x, chroma, 0]
    : hue < 3 ? [0, chroma, x]
    : hue < 4 ? [0, x, chroma]
    : hue < 5 ? [x, 0, chroma]
    : [chroma, 0, x];
  const m = l - chroma / 2;
  return toHex((r + m) * 255, (g + m) * 255, (b + m) * 255);
}

function toHex(...channels: number[]): string {
  return `#${channels
    .map((c) => Math.round(Math.min(255, Math.max(0, c))).toString(16).padStart(2, "0"))
    .join("")}`;
}
