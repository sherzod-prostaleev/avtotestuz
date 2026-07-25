import { readFileSync } from "node:fs";
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
      icons: unknown[];
    };
    expect(manifest.name).toBe("Driver Go");
    expect(manifest.short_name).toBe("Driver Go");
    expect(manifest.display).toBe("standalone");
    expect(manifest.start_url).toBe("/");
    expect(manifest.icons.length).toBeGreaterThan(0);
  });
});
