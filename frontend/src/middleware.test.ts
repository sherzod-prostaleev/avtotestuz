import fs from "node:fs";
import path from "node:path";
import { describe, it, expect, vi } from "vitest";
import { NextRequest, NextResponse } from "next/server";

vi.mock("next-intl/middleware", () => ({
  default: () => () => NextResponse.next(),
}));

import middleware from "./middleware";
import { AUTH_COOKIE } from "@/lib/auth-cookies";

function makeRequest(pathname: string, cookieHeader?: string): NextRequest {
  const headers = new Headers();
  if (cookieHeader) headers.set("cookie", cookieHeader);
  return new NextRequest(`http://localhost:3000${pathname}`, { headers });
}

describe("middleware auth guard", () => {
  it.each([
    "/uz-Latn/dashboard",
    "/uz-Latn/exam-mockup",
    "/uz-Latn/tickets",
    "/uz-Latn/practice",
    "/uz-Latn/image-questions",
    "/uz-Latn/mistakes",
    "/uz-Latn/signs",
    "/uz-Latn/stats",
    "/uz-Latn/profile",
    "/uz-Latn/premium",
    "/uz-Latn/saved",
    "/uz-Latn/session/start",
    "/uz-Latn/session/session-id",
  ])("redirects unauthenticated requests for %s to login", (pathname) => {
    const response = middleware(makeRequest(pathname));
    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe("http://localhost:3000/uz-Latn/login");
  });

  it("does not redirect a protected page when the access-token cookie is present", () => {
    const response = middleware(makeRequest("/uz-Latn/dashboard", `${AUTH_COOKIE}=some-token`));
    expect(response.headers.get("location")).toBeNull();
  });

  it("redirects an already-logged-in user away from the login page", () => {
    const response = middleware(makeRequest("/uz-Latn/login", `${AUTH_COOKIE}=some-token`));
    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe("http://localhost:3000/uz-Latn/dashboard");
  });

  it("does not touch the public landing page regardless of session state", () => {
    const withSession = middleware(makeRequest("/uz-Latn", `${AUTH_COOKIE}=some-token`));
    const withoutSession = middleware(makeRequest("/uz-Latn"));
    expect(withSession.headers.get("location")).toBeNull();
    expect(withoutSession.headers.get("location")).toBeNull();
  });

  it("delegates to next-intl untouched when the URL has no locale prefix yet", () => {
    const response = middleware(makeRequest("/dashboard"));
    expect(response.headers.get("location")).toBeNull(); // the mocked intl middleware returns NextResponse.next()
  });

  // PROTECTED_SEGMENTS is a hand-maintained list, so adding a route under
  // (app) without touching it silently ships an authenticated page that
  // anyone can open. Enumerate the directory instead of trusting memory.
  it("guards every route in the (app) group", () => {
    const appDir = path.join(__dirname, "app", "[locale]", "(app)");
    const routes = fs
      .readdirSync(appDir, { withFileTypes: true })
      .filter((entry) => entry.isDirectory())
      .map((entry) => entry.name);

    expect(routes.length).toBeGreaterThan(0);
    for (const route of routes) {
      const response = middleware(makeRequest(`/uz-Latn/${route}`));
      expect(response.headers.get("location"), `/${route} is not auth-guarded`).toBe(
        "http://localhost:3000/uz-Latn/login"
      );
    }
  });
});
