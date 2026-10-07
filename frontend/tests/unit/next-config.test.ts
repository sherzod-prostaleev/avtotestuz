import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

const config = fs.readFileSync(path.join(__dirname, "../../next.config.mjs"), "utf8");

describe("next.config media serving", () => {
  it("allows local MinIO over HTTP in development img-src", () => {
    expect(config).toContain("http://localhost:9000");
    expect(config).toContain("http://127.0.0.1:9000");
    expect(config).toMatch(/img-src 'self' data: blob: https:/);
  });

  it("rewrites same-origin /media to local MinIO in development", () => {
    expect(config).toContain("/media/:path*");
    expect(config).toContain("9000/media");
  });

  it("caches the chrome WebP as immutable", () => {
    expect(config).toContain("/logo-48.webp");
    expect(config).toContain("public, max-age=31536000, immutable");
  });
});

describe("next.config framing", () => {
  it("lets only Telegram Web frame the app", () => {
    expect(config).toContain("frame-ancestors 'self' https://web.telegram.org");
    expect(config).not.toContain('{ key: "X-Frame-Options", value: "DENY" },\n  { key: "Referrer-Policy"');
  });

  it("loads the Telegram SDK only from telegram.org", async () => {
    const mod = await import("../../next.config.mjs");
    const rules = await mod.default.headers!();
    const csp = rules.find((r) => r.source === "/:path*")!.headers.find((h) => h.key === "Content-Security-Policy")!.value;
    const scriptSrc = csp.split("; ").find((d) => d.startsWith("script-src "))!;
    expect(scriptSrc).toMatch(/^script-src 'self' 'unsafe-inline'( 'unsafe-eval')? https:\/\/static\.cloudflareinsights\.com https:\/\/telegram\.org$/);
  });

  it("keeps admin unframeable", () => {
    expect(config).toContain('source: "/:locale/admin/:path*"');
    expect(config).toContain('source: "/api/admin/:path*"');
    expect(config).toContain("frame-ancestors 'none'");
  });

  it("resolves per-path headers: learner framable by Telegram, admin never", async () => {
    const mod = await import("../../next.config.mjs");
    const rules = await mod.default.headers!();
    const headersFor = (source: string) => {
      const out = new Map<string, string>();
      for (const r of rules) {
        if (r.source === "/:path*" || r.source === source) {
          for (const h of r.headers) out.set(h.key, h.value); // later entry overwrites, as in Next
        }
      }
      return out;
    };
    const learner = headersFor("/:path*");
    expect(learner.get("Content-Security-Policy")).toContain(
      "frame-ancestors 'self' https://web.telegram.org",
    );
    expect(learner.has("X-Frame-Options")).toBe(false);
    expect(learner.get("Content-Security-Policy")).toMatch(/script-src [^;]*https:\/\/telegram\.org/);
    for (const src of ["/:locale/admin/:path*", "/api/admin/:path*"]) {
      const admin = headersFor(src);
      const csp = admin.get("Content-Security-Policy") ?? "";
      expect(csp).toContain("frame-ancestors 'none'");
      expect(csp).not.toContain("web.telegram.org");
      // The Telegram SDK is a learner-app dependency only; admin must not be
      // able to load scripts from telegram.org.
      expect(csp).not.toContain("https://telegram.org");
      expect(csp).toContain("script-src 'self'");
      expect(csp).toContain("default-src 'self'");
      expect(admin.get("X-Frame-Options")).toBe("DENY");
    }
  });
});
