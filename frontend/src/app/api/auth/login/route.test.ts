import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE, TG_MODE_COOKIE } from "@/lib/auth-cookies";

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

    expect(json).toEqual({ data: { ok: true, must_change_password: false } });
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

  it("issues Telegram-mode cookies and reports telegram_linked when the body carries tg_init_data", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({ data: { access_token: "abc.def", refresh_token: "xyz.123", telegram_linked: true } }),
        { status: 200 }
      )
    );
    vi.stubGlobal("fetch", fetchMock);
    const body = JSON.stringify({ phone: "901112233", password: "secret123", tg_init_data: "query_id=1&hash=ff" });

    const response = await POST(
      new Request("http://localhost/api/auth/login", {
        method: "POST",
        headers: { host: "drivergo.uz", origin: "https://drivergo.uz" },
        body,
      })
    );

    // The body reaches the backend untouched so it can link the account.
    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8090/api/v1/auth/login",
      expect.objectContaining({ body })
    );
    expect(await response.json()).toEqual({
      data: { ok: true, must_change_password: false, telegram_linked: true },
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
          new Response(JSON.stringify({ data: { access_token: "abc.def", refresh_token: "xyz.123" } }), { status: 200 })
        )
    );

    const response = await POST(
      new Request("http://localhost/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ phone: "901112233", password: "secret123", tg_init_data: "query_id=1&hash=ff" }),
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
          { status: 200 }
        )
      )
    );

    const response = await POST(
      new Request("http://localhost/api/auth/login", {
        method: "POST",
        body: JSON.stringify({ phone: "901112233", password: "secret123" }),
      })
    );

    expect(await response.json()).toEqual({ data: { ok: true, must_change_password: false } });
    const all = response.headers.getSetCookie().join("\n").toLowerCase();
    expect(all).toContain("samesite=lax");
    expect(all).not.toContain("partitioned");
    expect(all).not.toContain(`${TG_MODE_COOKIE}=`);
  });

  it("refuses a foreign Origin with 403 before calling the backend", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(
      new Request("http://localhost/api/auth/login", {
        method: "POST",
        headers: { host: "drivergo.uz", origin: "https://evil.example" },
        body: JSON.stringify({ phone: "901112233", password: "secret123" }),
      })
    );

    expect(response.status).toBe(403);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("keeps an existing Telegram jar partitioned without adding telegram_linked", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ data: { access_token: "abc.def", refresh_token: "xyz.123" } }), { status: 200 })
        )
    );

    const response = await POST(
      new Request("http://localhost/api/auth/login", {
        method: "POST",
        headers: { Cookie: `${TG_MODE_COOKIE}=1` },
        body: JSON.stringify({ phone: "901112233", password: "secret123" }),
      })
    );

    expect(await response.json()).toEqual({ data: { ok: true, must_change_password: false } });
    const cookies = response.headers.getSetCookie();
    expect(cookies).toHaveLength(3);
    for (const c of cookies) expect(c.toLowerCase()).toContain("partitioned");
  });
});
