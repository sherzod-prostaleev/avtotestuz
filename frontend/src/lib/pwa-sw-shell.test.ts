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

  it("keeps push handlers and caches metadata lists thinly", () => {
    expect(sw).toContain('addEventListener("push"');
    expect(sw).toContain('addEventListener("notificationclick"');
    expect(sw).toContain("META_LIST_RE");
    expect(sw).toContain("networkFirstMetaList");
    expect(sw).toContain("categories");
    expect(sw).toContain("signs");
    expect(sw).toContain("site");
    expect(sw).toContain("contacts");
    expect(sw).toContain('META_CACHE = "dg-meta-v2"');
    expect(sw).toContain('pathname.startsWith("/api/")');
    expect(sw).toContain('pathname.startsWith("/bff/")');
  });

  it("caches public legal/support shells after visit", () => {
    expect(sw).toContain("SHELL_PATH_RE");
    expect(sw).toContain("jarimalar");
    expect(sw).toContain("support");
  });

  it("caches recently opened variant/ticket detail payloads", () => {
    expect(sw).toContain("VARIANT_DETAIL_RE");
    expect(sw).toContain("networkFirstVariantDetail");
    expect(sw).toContain('VARIANT_CACHE = "dg-variant-v1"');
    expect(sw).toContain("VARIANT_CACHE_MAX");
    expect(sw).toContain("trimVariantCache");
    expect(sw).toMatch(/variants\\\/\\d+/);
  });

  it("does not claim full offline exam/content sync", () => {
    expect(sw).toMatch(/No full offline exam/);
    expect(sw).toMatch(/gap remains large/);
    expect(sw).not.toMatch(/IndexedDB|questions.?catalog/i);
  });
});
