import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE, TG_MODE_COOKIE } from "@/lib/auth-cookies";

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

  it("issues Telegram-mode cookies and reports telegram_linked when the body carries tg_init_data", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({ data: { access_token: "abc.def", refresh_token: "xyz.123", telegram_linked: true } }),
        { status: 201 }
      )
    );
    vi.stubGlobal("fetch", fetchMock);
    const body = JSON.stringify({ phone: "901112233", password: "secret123", name: "Ali", tg_init_data: "query_id=1&hash=ff" });

    const response = await POST(
      new Request("http://localhost/api/auth/register", {
        method: "POST",
        headers: { host: "drivergo.uz", origin: "https://drivergo.uz" },
        body,
      })
    );

    // The body reaches the backend untouched so it can link the account.
    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8090/api/v1/auth/register",
      expect.objectContaining({ body })
    );
    expect(await response.json()).toEqual({
      data: { ok: true, telegram_linked: true },
    });
    const all = response.headers.getSetCookie().join("\n").toLowerCase();
    expect(all).toContain("partitioned");
    expect(all).toContain("samesite=none");
    expect(all).toContain(`${TG_MODE_COOKIE}=1`);
  });

  it("reports telegram_linked false when the backend omits it in Telegram mode", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ data: { access_token: "abc.def", refresh_token: "xyz.123" } }), { status: 201 })
        )
    );

    const response = await POST(
      new Request("http://localhost/api/auth/register", {
        method: "POST",
        body: JSON.stringify({ phone: "901112233", password: "secret123", name: "Ali", tg_init_data: "query_id=1&hash=ff" }),
      })
    );

    expect((await response.json()).data.telegram_linked).toBe(false);
  });

  it("keeps lax cookies and the unchanged body without tg_init_data", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          // Even if the backend claimed a link, the website response shape stays as it was.
          JSON.stringify({ data: { access_token: "abc.def", refresh_token: "xyz.123", telegram_linked: true } }),
          { status: 201 }
        )
      )
    );

    const response = await POST(
      new Request("http://localhost/api/auth/register", {
        method: "POST",
        body: JSON.stringify({ phone: "901112233", password: "secret123", name: "Ali" }),
      })
    );

    expect(await response.json()).toEqual({ data: { ok: true } });
    const all = response.headers.getSetCookie().join("\n").toLowerCase();
    expect(all).toContain("samesite=lax");
    expect(all).not.toContain("partitioned");
    expect(all).not.toContain(`${TG_MODE_COOKIE}=`);
  });

  it("refuses a foreign Origin with 403 before calling the backend", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(
      new Request("http://localhost/api/auth/register", {
        method: "POST",
        headers: { host: "drivergo.uz", origin: "https://evil.example" },
        body: JSON.stringify({ phone: "901112233", password: "secret123", name: "Ali" }),
      })
    );

    expect(response.status).toBe(403);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
