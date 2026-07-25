import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

describe("PWA offline shell service worker", () => {
  const sw = readFileSync(join(process.cwd(), "public/sw.js"), "utf8");
  const offline = readFileSync(join(process.cwd(), "public/offline.html"), "utf8");

  it("precaches shell assets and offline fallback", () => {
    expect(sw).toContain('SHELL_CACHE = "dg-shell-v2"');
    expect(sw).toContain('OFFLINE_URL = "/offline.html"');
    expect(sw).toContain("/manifest.webmanifest");
    expect(sw).toContain("cache.addAll(PRECACHE_URLS)");
    expect(sw).toContain("networkFirstNavigation");
    expect(offline).toContain("Driver Go");
    expect(offline).toContain("Internet aloqasi");
  });

  it("keeps push handlers and caches bilets list API thinly", () => {
    expect(sw).toContain('addEventListener("push"');
    expect(sw).toContain('addEventListener("notificationclick"');
    expect(sw).toContain("BILETS_LIST_RE");
    expect(sw).toContain("networkFirstBiletsList");
    expect(sw).toContain('pathname.startsWith("/api/")');
    expect(sw).toContain('pathname.startsWith("/bff/")');
  });

  it("does not claim full offline exam/content sync", () => {
    expect(sw).toMatch(/No full offline exam/);
    expect(sw).not.toMatch(/IndexedDB|questions.?catalog/i);
  });
});
