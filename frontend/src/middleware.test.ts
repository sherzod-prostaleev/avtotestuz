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
  it("redirects to login when a protected page is requested without a session cookie", () => {
    const response = middleware(makeRequest("/uz-Latn/dashboard"));
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
});
