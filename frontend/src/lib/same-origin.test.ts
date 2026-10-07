import { describe, expect, it } from "vitest";
import { rejectCrossSite } from "./same-origin";

function req(method: string, headers: Record<string, string>) {
  return new Request("http://internal:3000/api/proxy/x", { method, headers: { host: "drivergo.uz", ...headers } });
}

describe("rejectCrossSite", () => {
  it("lets safe methods through untouched", () => {
    expect(rejectCrossSite(req("GET", { origin: "https://evil.example" }))).toBeNull();
    expect(rejectCrossSite(req("HEAD", { origin: "https://evil.example" }))).toBeNull();
  });

  it("allows same-host Origin", () => {
    expect(rejectCrossSite(req("POST", { origin: "https://drivergo.uz" }))).toBeNull();
    expect(rejectCrossSite(req("PATCH", { origin: "https://drivergo.uz" }))).toBeNull();
  });

  it("allows the dev/e2e server, whose Host carries the port", () => {
    const local = new Request("http://localhost:3112/api/auth/login", {
      method: "POST",
      headers: { host: "localhost:3112", origin: "http://localhost:3112" },
    });
    expect(rejectCrossSite(local)).toBeNull();
  });

  it("compares hosts case-insensitively", () => {
    expect(rejectCrossSite(req("POST", { host: "DriverGo.uz", origin: "https://drivergo.uz" }))).toBeNull();
  });

  it("blocks a foreign Origin with 403 cross_site", async () => {
    const res = rejectCrossSite(req("POST", { origin: "https://evil.example" }))!;
    expect(res.status).toBe(403);
    expect((await res.json()).error.code).toBe("cross_site");
  });

  it("blocks a sibling subdomain and a different port", () => {
    expect(rejectCrossSite(req("POST", { origin: "https://www.drivergo.uz" }))?.status).toBe(403);
    expect(rejectCrossSite(req("POST", { origin: "https://drivergo.uz:8443" }))?.status).toBe(403);
  });

  it("blocks Origin: null (sandboxed frames)", () => {
    expect(rejectCrossSite(req("DELETE", { origin: "null" }))?.status).toBe(403);
  });

  it("blocks an Origin when the request carries no Host to compare with", () => {
    const noHost = new Request("http://internal:3000/api/proxy/x", {
      method: "POST",
      headers: { origin: "https://drivergo.uz" },
    });
    noHost.headers.delete("host");
    expect(rejectCrossSite(noHost)?.status).toBe(403);
  });

  it("requires the Origin scheme to match X-Forwarded-Proto when the proxy sends it", () => {
    expect(
      rejectCrossSite(req("POST", { origin: "https://drivergo.uz", "x-forwarded-proto": "https" }))
    ).toBeNull();
    expect(
      rejectCrossSite(req("POST", { origin: "http://drivergo.uz", "x-forwarded-proto": "https" }))?.status
    ).toBe(403);
    expect(
      rejectCrossSite(req("POST", { origin: "https://drivergo.uz", "x-forwarded-proto": "http" }))?.status
    ).toBe(403);
    // A proxy chain may append; the first hop is the client-facing scheme.
    expect(
      rejectCrossSite(req("POST", { origin: "https://drivergo.uz", "x-forwarded-proto": "HTTPS, http" }))
    ).toBeNull();
  });

  it("falls back to host-only when no X-Forwarded-Proto is present", () => {
    expect(rejectCrossSite(req("POST", { origin: "http://drivergo.uz" }))).toBeNull();
  });

  it("without Origin, blocks Sec-Fetch-Site cross-site and allows the rest", () => {
    expect(rejectCrossSite(req("POST", { "sec-fetch-site": "cross-site" }))?.status).toBe(403);
    expect(rejectCrossSite(req("POST", { "sec-fetch-site": "same-origin" }))).toBeNull();
    expect(rejectCrossSite(req("POST", {}))).toBeNull();
  });
});
