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
