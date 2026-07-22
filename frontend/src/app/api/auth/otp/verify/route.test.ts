import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("POST /api/auth/otp/verify", () => {
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

    const request = new Request("http://localhost/api/auth/otp/verify", {
      method: "POST",
      body: JSON.stringify({ phone: "901112233", code: "123456" }),
    });
    const response = await POST(request);
    const json = await response.json();

    expect(json).toEqual({ data: { ok: true } });
    expect(JSON.stringify(json)).not.toContain("abc.def");
    expect(JSON.stringify(json)).not.toContain("xyz.123");

    const atCookie = response.cookies.get(AUTH_COOKIE);
    const rtCookie = response.cookies.get(REFRESH_COOKIE);
    expect(atCookie?.value).toBe("abc.def");
    expect(atCookie?.httpOnly).toBe(true);
    expect(rtCookie?.value).toBe("xyz.123");
    expect(rtCookie?.httpOnly).toBe(true);
  });

  it("passes through invalid_code without setting any cookies", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ error: { code: "invalid_code", message: "wrong code" } }), { status: 400 })
        )
    );

    const request = new Request("http://localhost/api/auth/otp/verify", {
      method: "POST",
      body: JSON.stringify({ phone: "901112233", code: "000000" }),
    });
    const response = await POST(request);
    const json = await response.json();

    expect(response.status).toBe(400);
    expect(json.error.code).toBe("invalid_code");
    expect(response.cookies.get(AUTH_COOKIE)).toBeUndefined();
  });
});
