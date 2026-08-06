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

/** Absolute paths to every page.tsx under (kiosk). */
function findPageFiles(dir: string): string[] {
  const files: string[] = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const entryPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...findPageFiles(entryPath));
    } else if (entry.name === "page.tsx") {
      files.push(entryPath);
    }
  }
  return files;
}

/**
 * A kiosk wrapper is three lines; the navigation that can strand a student
 * lives in the `(app)` page it reuses. This resolves that import so the
 * scan below covers both files.
 *
 * Only literal paths are checked. A target built at runtime — from an API
 * response, or a variable this scan cannot see — is invisible here, and no
 * static check can fix that. Those are covered by the per-page "kiosk mode"
 * assertions in each reused page's own test file.
 */
function reusedPageFiles(kioskPageFile: string): string[] {
  const src = fs.readFileSync(kioskPageFile, "utf8");
  const out: string[] = [];
  for (const m of src.matchAll(/from\s+"@\/app\/([^"]+)"/g)) {
    const candidate = path.join(kioskDir, "..", "..", "..", "app", m[1] + ".tsx");
    if (fs.existsSync(candidate)) out.push(candidate);
  }
  return out;
}

/**
 * Literal navigation targets: href="/..." , router.push("/...") , router.replace("/...")
 * Each target carries the 0-based line it was found on, so the offender scan
 * below can check that line (and the one above it) for a kiosk-safe marker.
 */
function navigationTargets(file: string): { target: string; line: number }[] {
  const src = fs.readFileSync(file, "utf8");
  const lineOf = (index: number) => src.slice(0, index).split("\n").length - 1;
  const out: { target: string; line: number }[] = [];
  for (const m of src.matchAll(/(?:href=|router\.(?:push|replace)\()\s*[`"']([^`"']*)[`"']/g)) {
    out.push({ target: m[1], line: lineOf(m.index!) });
  }
  for (const m of src.matchAll(/(?:href=|router\.(?:push|replace)\()\s*\{?\s*`([^`]*)`/g)) {
    out.push({ target: m[1], line: lineOf(m.index!) });
  }
  return out;
}

/**
 * Escape hatch for a literal target the scan cannot know is dead code: a
 * `!kiosk &&` guard or an `if (kiosk) return` a few lines up keeps it from
 * ever rendering or firing for a walk-up student, but this scan is lexical
 * — it does not evaluate control flow, so a genuinely gated literal looks
 * identical to a live one. Marking the site `kiosk-safe: <reason>` opts it
 * out, but only when a reason follows: a bare marker with nothing after it
 * doesn't count (the length check below), so silencing a target costs
 * exactly as much as explaining it. A target with no marker still fails —
 * this is a per-line claim, not a blanket suppression, and every use is a
 * promise that a runtime "kiosk mode" test (see each reused page's own test
 * file) actually renders or exercises that code path with kiosk=true and
 * confirms the guard holds. The static scan has no way to verify that
 * promise — only that the marker and a real reason are present.
 */
function isMarkedKioskSafe(lines: string[], line: number): boolean {
  const marker = /kiosk-safe:\s*(.*)$/;
  for (const candidate of [lines[line], lines[line - 1]]) {
    const reason = candidate?.match(marker)?.[1]?.trim();
    if (reason && reason.length >= 10) return true;
  }
  return false;
}

/**
 * `/${locale}/station/practice` and `/uz-Latn/practice` both need to reduce
 * to the path the middleware sees. Strips a leading template locale
 * placeholder or a literal locale segment, and any query string.
 */
function toMiddlewarePath(target: string): string | null {
  if (!target.startsWith("/")) return null;
  const noQuery = target.split("?")[0];
  const stripped = noQuery
    .replace(/^\/\$\{locale\}/, "")
    .replace(/^\/(uz-Latn|uz-Cyrl|ru)(?=\/|$)/, "");
  return stripped === "" ? "/" : stripped;
}

describe("kiosk-safe marker", () => {
  // Guards the escape hatch itself: if a bare marker suppressed a finding,
  // silencing a real navigation break would cost nothing.
  it("requires an actual reason, not just the bare marker", () => {
    expect(isMarkedKioskSafe(["href={x} // kiosk-safe:"], 0)).toBe(false);
    expect(isMarkedKioskSafe(["// kiosk-safe: x", "href={x}"], 1)).toBe(false);
    expect(isMarkedKioskSafe(["href={x}"], 0)).toBe(false);
  });

  it("suppresses only when a real reason is on the target's own line or the one above", () => {
    expect(isMarkedKioskSafe(["router.push(x); // kiosk-safe: unreachable, guarded above"], 0)).toBe(true);
    expect(isMarkedKioskSafe(["// kiosk-safe: wrapped in !kiosk two lines up", "href={x}"], 1)).toBe(true);
    // Two lines up doesn't count — only the match line or the one directly above it.
    expect(isMarkedKioskSafe(["// kiosk-safe: wrapped in !kiosk", "<Link", "href={x}"], 2)).toBe(false);
  });
});

describe("kiosk navigation targets", () => {
  it("never navigates to a cookie-gated route", () => {
    const pageFiles = findPageFiles(kioskDir);
    expect(pageFiles.length).toBeGreaterThan(0);

    const offenders: string[] = [];
    for (const pageFile of pageFiles) {
      const files = [pageFile, ...reusedPageFiles(pageFile)];
      for (const file of files) {
        const lines = fs.readFileSync(file, "utf8").split("\n");
        for (const { target, line } of navigationTargets(file)) {
          const p = toMiddlewarePath(target);
          if (p && matchesAny(p, PROTECTED_SEGMENTS) && !isMarkedKioskSafe(lines, line)) {
            offenders.push(`${path.relative(kioskDir, file)} -> ${target}`);
          }
        }
      }
    }
    expect(offenders).toEqual([]);
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
