import fs from "node:fs";
import path from "node:path";
import { describe, it, expect, vi } from "vitest";
import { NextRequest, NextResponse } from "next/server";

vi.mock("next-intl/middleware", () => ({
  default: () => () => NextResponse.next(),
}));

import proxy from "./proxy";
import { AUTH_COOKIE } from "@/lib/auth-cookies";

function makeRequest(pathname: string, cookieHeader?: string): NextRequest {
  const headers = new Headers();
  if (cookieHeader) headers.set("cookie", cookieHeader);
  return new NextRequest(`http://localhost:3000${pathname}`, { headers });
}

describe("proxy auth guard", () => {
  it.each([
    "/uz-Latn/dashboard",
    "/uz-Latn/exam-mockup",
    "/uz-Latn/tickets",
    "/uz-Latn/practice",
    "/uz-Latn/mistakes",
    "/uz-Latn/signs",
    "/uz-Latn/stats",
    "/uz-Latn/profile",
    "/uz-Latn/premium",
    "/uz-Latn/saved",
    "/uz-Latn/session/start",
    "/uz-Latn/session/session-id",
  ])("redirects unauthenticated requests for %s to login", (pathname) => {
    const response = proxy(makeRequest(pathname));
    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe("http://localhost:3000/uz-Latn/login");
  });

  it("does not redirect a protected page when the access-token cookie is present", () => {
    const response = proxy(makeRequest("/uz-Latn/dashboard", `${AUTH_COOKIE}=some-token`));
    expect(response.headers.get("location")).toBeNull();
  });

  it("redirects an already-logged-in user away from the login page", () => {
    const response = proxy(makeRequest("/uz-Latn/login", `${AUTH_COOKIE}=some-token`));
    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe("http://localhost:3000/uz-Latn/dashboard");
  });

  it("keeps reset-password public so Telegram round-trip can finish", () => {
    expect(proxy(makeRequest("/uz-Latn/reset-password?token=abc")).headers.get("location")).toBeNull();
    expect(
      proxy(makeRequest("/uz-Latn/reset-password?token=abc", `${AUTH_COOKIE}=some-token`)).headers.get("location")
    ).toBeNull();
  });

  it("redirects an already-logged-in user away from register and forgot-password", () => {
    expect(proxy(makeRequest("/uz-Latn/register", `${AUTH_COOKIE}=some-token`)).headers.get("location")).toBe(
      "http://localhost:3000/uz-Latn/dashboard"
    );
    expect(
      proxy(makeRequest("/uz-Latn/forgot-password", `${AUTH_COOKIE}=some-token`)).headers.get("location")
    ).toBe("http://localhost:3000/uz-Latn/dashboard");
  });

  // An invite link (/r/{code} -> /login?ref=CODE) opened by someone who is
  // already signed in gets bounced off /login. Dropping the query here loses
  // the referral outright, since nothing downstream can recover the code.
  it("carries ?ref across the login redirect for an authenticated visitor", () => {
    const response = proxy(makeRequest("/uz-Latn/login?ref=FRIEND01", `${AUTH_COOKIE}=some-token`));
    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe("http://localhost:3000/uz-Latn/dashboard?ref=FRIEND01");
  });

  it("forwards only ref, not arbitrary query params, on that redirect", () => {
    const response = proxy(
      makeRequest("/uz-Latn/login?ref=FRIEND01&next=%2Fevil", `${AUTH_COOKIE}=some-token`)
    );
    expect(response.headers.get("location")).toBe("http://localhost:3000/uz-Latn/dashboard?ref=FRIEND01");
  });

  it("does not touch the public landing page regardless of session state", () => {
    const withSession = proxy(makeRequest("/uz-Latn", `${AUTH_COOKIE}=some-token`));
    const withoutSession = proxy(makeRequest("/uz-Latn"));
    expect(withSession.headers.get("location")).toBeNull();
    expect(withoutSession.headers.get("location")).toBeNull();
  });

  it("delegates to next-intl untouched when the URL has no locale prefix yet", () => {
    const response = proxy(makeRequest("/dashboard"));
    expect(response.headers.get("location")).toBeNull(); // the mocked intl middleware returns NextResponse.next()
  });

  it.each([
    ["health", "monitoring/health"],
    ["providers", "payments/providers"],
    ["contacts", "cms/chrome"],
    ["users", "users"],
    ["payments", "payments/transactions"],
    ["audit", "security/audit"],
    ["limits", "settings/limits"],
  ])("moves legacy shared-token Ops %s to admin RBAC", (legacy, adminPath) => {
    const response = proxy(makeRequest(`/uz-Latn/ops/${legacy}`));
    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(
      `http://localhost:3000/uz-Latn/admin/${adminPath}`,
    );
  });

  // PROTECTED_SEGMENTS is a hand-maintained list, so adding a route without
  // touching it silently ships an authenticated page anyone can open.
  // Enumerate the route groups instead of trusting memory — this also keeps
  // working when a route moves between groups, as /session did when the test
  // screen went full-screen.
  it("guards every route outside the public, auth and kiosk groups", () => {
    const localeDir = path.join(__dirname, "app", "[locale]");
    // (kiosk) holds /station: a login-free classroom landing page opened on
    // a shared PC that never carries a learner AUTH_COOKIE. Gating it behind
    // /login would make it unreachable from the very PCs it exists for. The
    // page itself checks GET /me's `kind` and renders a refusal for anything
    // that isn't a station, so it belongs with (public)/(auth) here, not in
    // the exhaustive gated-route loop below.
    const publicGroups = new Set(["(public)", "(auth)", "(kiosk)"]);
    const groups = fs
      .readdirSync(localeDir, { withFileTypes: true })
      .filter((entry) => entry.isDirectory() && entry.name.startsWith("("))
      .filter((entry) => !publicGroups.has(entry.name));

    const routes = groups.flatMap((group) =>
      fs
        .readdirSync(path.join(localeDir, group.name), { withFileTypes: true })
        .filter((entry) => entry.isDirectory())
        .map((entry) => entry.name)
    );

    expect(routes).toContain("session");
    expect(routes.length).toBeGreaterThan(1);
    expect(routes).not.toContain("station");
    for (const route of routes) {
      const response = proxy(makeRequest(`/uz-Latn/${route}`));
      expect(response.headers.get("location"), `/${route} is not auth-guarded`).toBe(
        "http://localhost:3000/uz-Latn/login"
      );
    }
  });

  it("does not gate the login-free classroom kiosk page", () => {
    const response = proxy(makeRequest("/uz-Latn/station"));
    expect(response.headers.get("location")).toBeNull();
  });
});
