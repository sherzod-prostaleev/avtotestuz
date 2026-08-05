import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { PROTECTED_SEGMENTS, matchesAny } from "@/lib/protected-segments";

const kioskDir = path.dirname(fileURLToPath(import.meta.url));

/**
 * Every `page.tsx` under the (kiosk) route group is a route a walk-up
 * student on the classroom PC can navigate to — the agent proxies
 * `/api/proxy/*` with a station token, but the browser itself holds no
 * auth cookie at all. This walks the filesystem instead of hand-listing
 * routes so a future station/* page that lands under a gated segment (the
 * exact class of bug fixed three times in a row on this branch: /station
 * -> /station/practice+tickets -> /session/start -> /session/[id]) fails a
 * test instead of shipping.
 */
function findPageRoutes(dir: string, segments: string[] = []): string[] {
  const routes: string[] = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.name.endsWith(".test.tsx") || entry.name.endsWith(".test.ts")) continue;
    const entryPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      routes.push(...findPageRoutes(entryPath, [...segments, entry.name]));
    } else if (entry.name === "page.tsx") {
      routes.push("/" + segments.join("/"));
    }
  }
  return routes;
}

/**
 * not-found.tsx and error.tsx are boundaries, not routes — they own no URL
 * segment of their own, so they're invisible to findPageRoutes above (which
 * only looks for "page.tsx") and to the PROTECTED_SEGMENTS check that
 * follows. They're exactly the surface a walk-up student lands on when a
 * route is missing or throws, so this walks the filesystem for them the
 * same way, and the test below reads their source to confirm the one
 * escape hatch they offer stays inside the kiosk.
 */
function findBoundaryFiles(dir: string): string[] {
  const files: string[] = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.name.endsWith(".test.tsx") || entry.name.endsWith(".test.ts")) continue;
    const entryPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...findBoundaryFiles(entryPath));
    } else if (entry.name === "not-found.tsx" || entry.name === "error.tsx") {
      files.push(entryPath);
    }
  }
  return files;
}

describe("kiosk route registration", () => {
  it("keeps every (kiosk) page.tsx outside PROTECTED_SEGMENTS", () => {
    const routes = findPageRoutes(kioskDir);

    // Guards the guard: an empty list would make the loop below vacuously
    // pass and silently stop catching anything.
    expect(routes).toEqual(
      expect.arrayContaining([
        "/station",
        "/station/practice",
        "/station/tickets",
        "/station/session/start",
        "/station/session/[id]",
      ])
    );

    for (const route of routes) {
      expect(matchesAny(route, PROTECTED_SEGMENTS)).toBe(false);
    }
  });

  it("keeps /station itself, and the /station namespace generally, unprotected", () => {
    // The middleware exemption this whole feature depends on: PROTECTED_SEGMENTS
    // must never gain "station" (that's the one segment this feature owns),
    // and none of its existing entries may accidentally prefix-match it.
    expect(PROTECTED_SEGMENTS).not.toContain("station");
    expect(matchesAny("/station", PROTECTED_SEGMENTS)).toBe(false);
    expect(matchesAny("/station/practice", PROTECTED_SEGMENTS)).toBe(false);
    expect(matchesAny("/station/tickets", PROTECTED_SEGMENTS)).toBe(false);
    expect(matchesAny("/station/session/start", PROTECTED_SEGMENTS)).toBe(false);
    expect(matchesAny("/station/session/abc-123", PROTECTED_SEGMENTS)).toBe(false);

    // And the learner-namespace equivalents stay protected — the kiosk must
    // reach its own /station/... routes, never unauthenticate the shared ones.
    expect(matchesAny("/practice", PROTECTED_SEGMENTS)).toBe(true);
    expect(matchesAny("/tickets", PROTECTED_SEGMENTS)).toBe(true);
    expect(matchesAny("/session/start", PROTECTED_SEGMENTS)).toBe(true);
    expect(matchesAny("/session/abc-123", PROTECTED_SEGMENTS)).toBe(true);
  });
});

describe("kiosk error boundaries", () => {
  it("gives the (kiosk) group its own not-found and error boundaries", () => {
    const boundaries = findBoundaryFiles(kioskDir).map((f) => path.relative(kioskDir, f));

    // Next.js resolves the nearest boundary. Without a kiosk-scoped pair,
    // a stranded student falls through to the shared [locale]/not-found.tsx
    // and [locale]/error.tsx, which link back to the public marketing site.
    expect(boundaries.sort()).toEqual(["error.tsx", "not-found.tsx"]);
  });

  it("links each kiosk boundary only to /station, never to dashboard/premium/checkout/etc", () => {
    const boundaries = findBoundaryFiles(kioskDir);
    expect(boundaries.length).toBeGreaterThan(0);

    for (const file of boundaries) {
      const source = fs.readFileSync(file, "utf8");
      const hrefs = [...source.matchAll(/href=\{`([^`]*)`\}/g)].map((m) => m[1]);

      // Exactly one link out, and it goes to the kiosk home — nothing else.
      expect(hrefs).toEqual(["/${locale}/station"]);
    }
  });
});
