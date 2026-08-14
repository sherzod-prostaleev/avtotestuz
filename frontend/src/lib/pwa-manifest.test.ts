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
    expect(manifest.icons.some((i) => i.type === "image/svg+xml")).toBe(false);
    expect(existsSync(join(process.cwd(), "public/logo-512.png"))).toBe(true);
    expect(existsSync(join(process.cwd(), "public/logo-48.webp"))).toBe(true);
    expect(statSync(join(process.cwd(), "public/logo-48.webp")).size).toBeLessThan(10_000);
    expect(existsSync(join(process.cwd(), "public/apple-touch-icon.png"))).toBe(true);
    expect(existsSync(join(process.cwd(), "public/favicon.ico"))).toBe(true);
    expect(existsSync(join(process.cwd(), "public/favicon-32.png"))).toBe(true);
    expect(existsSync(join(process.cwd(), "public/favicon-16.png"))).toBe(true);
    expect(existsSync(join(process.cwd(), "src/app/icon.png"))).toBe(true);
    expect(existsSync(join(process.cwd(), "src/app/icon.svg"))).toBe(false);
    const svg = readFileSync(join(process.cwd(), "public/logo.svg"), "utf8");
    expect(svg).toMatch(/<svg[\s>]/);
    expect(svg).toContain("/logo-512.png");
    expect(svg).not.toMatch(/data:image|base64,/);
    expect(statSync(join(process.cwd(), "public/logo.svg")).size).toBeLessThan(8_000);
    expect(statSync(join(process.cwd(), "public/logo-512.png")).size).toBeLessThan(120_000);
    expect(statSync(join(process.cwd(), "public/apple-touch-icon.png")).size).toBeLessThan(80_000);
    expect(statSync(join(process.cwd(), "public/favicon.ico")).size).toBeLessThan(40_000);
    expect(statSync(join(process.cwd(), "public/favicon-32.png")).size).toBeLessThan(8_000);
    for (const icon of manifest.icons) {
      expect(existsSync(join(process.cwd(), "public", icon.src.replace(/^\//, "")))).toBe(true);
    }
  });

  it("does not advertise an SVG tab icon that browsers would prefer over the 3D raster", () => {
    const layout = readFileSync(join(process.cwd(), "src/app/[locale]/layout.tsx"), "utf8");
    expect(layout).not.toMatch(/image\/svg\+xml/);
    expect(layout).toContain("/favicon-32.png");
    expect(layout).toContain("/favicon.ico");
  });
});
