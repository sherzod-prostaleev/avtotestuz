import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("POST /api/auth/login", () => {
  it("sets httpOnly auth cookies and never exposes tokens in the response body", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ data: { access_token: "abc.def", refresh_token: "xyz.123" } }), {
            status: 200,
          })
        )
    );

    const request = new Request("http://localhost/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ phone: "901112233", password: "secret123" }),
    });
    const response = await POST(request);
    const json = await response.json();

    expect(json).toEqual({ data: { ok: true } });
    expect(JSON.stringify(json)).not.toContain("abc.def");
    expect(JSON.stringify(json)).not.toContain("secret123");

    const atCookie = response.cookies.get(AUTH_COOKIE);
    const rtCookie = response.cookies.get(REFRESH_COOKIE);
    expect(atCookie?.value).toBe("abc.def");
    expect(atCookie?.httpOnly).toBe(true);
    expect(rtCookie?.value).toBe("xyz.123");
  });

  it("passes through invalid_credentials without setting cookies", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ error: { code: "invalid_credentials", message: "invalid phone or password" } }), {
            status: 401,
          })
        )
    );

    const request = new Request("http://localhost/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ phone: "901112233", password: "wrongpass" }),
    });
    const response = await POST(request);
    const json = await response.json();

    expect(response.status).toBe(401);
    expect(json.error.code).toBe("invalid_credentials");
    expect(response.cookies.get(AUTH_COOKIE)).toBeUndefined();
  });
});
