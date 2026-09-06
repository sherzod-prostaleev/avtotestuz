import { existsSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const frontendRoot = process.cwd();
const repoRoot = join(frontendRoot, "..");

describe("origin speed plan contracts", () => {
  it("ensures landing page has valid navigation links", () => {
    const src = readFileSync(
      join(frontendRoot, "src/app/[locale]/(public)/home-client.tsx"),
      "utf8",
    );
    const linkCount = (src.match(/<Link\b/g) ?? []).length;
    expect(linkCount).toBeGreaterThan(0);
  });


  it("loads the official exam chrome as a client chunk", () => {
    const src = readFileSync(
      join(frontendRoot, "src/app/[locale]/(session)/session/[id]/page.tsx"),
      "utf8",
    );
    expect(src).toContain('from "next/dynamic"');
    expect(src).toContain("official-avtotest-exam-view");
    expect(src).toContain("exam-pass-celebration");
    expect(src).not.toMatch(
      /import \{ OfficialAvtotestExamView \} from "@\/components\/exam\/official-avtotest-exam-view"/,
    );
  });

  it("keeps a session loading shell but none for (app)", () => {
    // Starting a session POSTs before it can render, so that wait gets a shell.
    expect(existsSync(join(frontendRoot, "src/app/[locale]/(session)/loading.tsx"))).toBe(true);

    // (app) deliberately has none. Its RSC fetch is ~180ms and every page draws
    // its own skeleton, so a route-level spinner only flashed. Worse, a Suspense
    // fallback made template.tsx mount around the *spinner*: the fade played on
    // the spinner and the real content then replaced it with no animation.
    expect(existsSync(join(frontendRoot, "src/app/[locale]/(app)/loading.tsx"))).toBe(false);
    expect(existsSync(join(frontendRoot, "src/app/[locale]/(app)/template.tsx"))).toBe(true);

    expect(readFileSync(join(frontendRoot, "src/app/[locale]/(app)/layout.tsx"), "utf8")).not.toMatch(
      /^"use client";/m,
    );
  });

  it("keeps the exam placeholder URL and a small chrome WebP", () => {
    const placeholder = join(frontendRoot, "public/exam/placeholder-driver-go-cars.png");
    expect(existsSync(placeholder)).toBe(true);
    expect(statSync(join(frontendRoot, "public/logo-48.webp")).size).toBeLessThan(10_000);
    const resolver = readFileSync(join(frontendRoot, "src/lib/question-image.ts"), "utf8");
    expect(resolver).toContain("/exam/placeholder-driver-go-cars.png");
  });

  it("uses a cheap web liveness probe in compose", () => {
    for (const rel of [
      "deploy/docker-compose.prod.yml",
      "deploy/docker-compose.app.yml",
      "deploy/docker-compose.candidate.yml",
    ]) {
      const text = readFileSync(join(repoRoot, rel), "utf8");
      expect(text).toContain("/api/healthz");
      expect(text).not.toContain("127.0.0.1:3000/uz-Latn");
    }
  });
});
