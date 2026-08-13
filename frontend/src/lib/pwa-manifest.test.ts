import { existsSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

describe("PWA manifest", () => {
  it("declares installable standalone app metadata", () => {
    const raw = readFileSync(join(process.cwd(), "public/manifest.webmanifest"), "utf8");
    const manifest = JSON.parse(raw) as {
      name: string;
      short_name: string;
      display: string;
      start_url: string;
      icons: Array<{ src: string; type?: string; sizes?: string }>;
    };
    expect(manifest.name).toBe("Driver Go");
    expect(manifest.short_name).toBe("Driver Go");
    expect(manifest.display).toBe("standalone");
    expect(manifest.start_url).toBe("/");
    expect(manifest.icons.length).toBeGreaterThan(0);
    expect(manifest.icons.some((i) => i.src === "/logo-512.png" && i.type === "image/png")).toBe(
      true,
    );
    expect(existsSync(join(process.cwd(), "public/logo-512.png"))).toBe(true);
    expect(existsSync(join(process.cwd(), "public/apple-touch-icon.png"))).toBe(true);
    expect(existsSync(join(process.cwd(), "public/favicon.ico"))).toBe(true);
    expect(existsSync(join(process.cwd(), "public/logo.svg"))).toBe(true);
    const svg = readFileSync(join(process.cwd(), "public/logo.svg"), "utf8");
    expect(svg).toMatch(/<svg[\s>]/);
    expect(svg).not.toMatch(/image\/png|data:image|base64,/);
    expect(statSync(join(process.cwd(), "public/logo.svg")).size).toBeLessThan(8_000);
    expect(statSync(join(process.cwd(), "public/logo-512.png")).size).toBeLessThan(120_000);
    expect(statSync(join(process.cwd(), "public/apple-touch-icon.png")).size).toBeLessThan(80_000);
    expect(statSync(join(process.cwd(), "public/favicon.ico")).size).toBeLessThan(40_000);
    expect(readFileSync(join(process.cwd(), "src/app/icon.svg"), "utf8")).toBe(svg);
    for (const icon of manifest.icons) {
      expect(existsSync(join(process.cwd(), "public", icon.src.replace(/^\//, "")))).toBe(true);
    }
  });
});
