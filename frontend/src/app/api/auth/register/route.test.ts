import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("POST /api/auth/register", () => {
  it("sets cookies on success and returns 201", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ data: { access_token: "abc.def", refresh_token: "xyz.123" } }), {
            status: 201,
          })
        )
    );

    const request = new Request("http://localhost/api/auth/register", {
      method: "POST",
      body: JSON.stringify({ phone: "901112233", password: "secret123", name: "Ali" }),
    });
    const response = await POST(request);
    const json = await response.json();

    expect(response.status).toBe(201);
    expect(json).toEqual({ data: { ok: true } });
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("abc.def");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("xyz.123");
  });

  it("passes through phone_taken without setting cookies", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ error: { code: "phone_taken", message: "taken" } }), { status: 409 })
        )
    );

    const request = new Request("http://localhost/api/auth/register", {
      method: "POST",
      body: JSON.stringify({ phone: "901112233", password: "secret123" }),
    });
    const response = await POST(request);

    expect(response.status).toBe(409);
    expect((await response.json()).error.code).toBe("phone_taken");
    expect(response.cookies.get(AUTH_COOKIE)).toBeUndefined();
  });
});
