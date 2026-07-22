import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";

afterEach(() => {
  vi.unstubAllGlobals();
});

function requestWithCookie(cookieHeader?: string): Request {
  const headers: Record<string, string> = cookieHeader ? { Cookie: cookieHeader } : {};
  return new Request("http://localhost/api/auth/refresh", { method: "POST", headers });
}

describe("POST /api/auth/refresh", () => {
  it("rotates cookies on a successful backend refresh", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ data: { access_token: "new-at", refresh_token: "new-rt" } }), { status: 200 })
        )
    );

    const response = await POST(requestWithCookie("rt=old-rt"));

    expect(response.status).toBe(200);
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("new-at");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("new-rt");
  });

  it("returns invalid_refresh and clears cookies when no refresh cookie is present", async () => {
    const response = await POST(requestWithCookie(undefined));
    const json = await response.json();

    expect(response.status).toBe(401);
    expect(json.error.code).toBe("invalid_refresh");
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("");
  });

  it("returns invalid_refresh and clears cookies when the backend rejects the refresh token", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(new Response(JSON.stringify({ error: { code: "refresh_reused" } }), { status: 401 }))
    );

    const response = await POST(requestWithCookie("rt=stolen-rt"));
    const json = await response.json();

    expect(response.status).toBe(401);
    expect(json.error.code).toBe("invalid_refresh");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("");
  });
});
