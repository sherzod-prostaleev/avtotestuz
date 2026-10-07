import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE, TG_MODE_COOKIE } from "@/lib/auth-cookies";

afterEach(() => {
  vi.unstubAllGlobals();
});

function telegramRequest(headers: Record<string, string> = {}): Request {
  return new Request("http://localhost/api/auth/telegram", {
    method: "POST",
    headers: { host: "drivergo.uz", "content-type": "application/json", ...headers },
    body: JSON.stringify({ init_data: "query_id=1&user=%7B%7D&auth_date=1&hash=ff" }),
  });
}

describe("POST /api/auth/telegram", () => {
  it("exchanges init data for Telegram-mode cookies without exposing tokens", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: { access_token: "abc.def", refresh_token: "xyz.123", must_change_password: true, telegram_linked: true },
        }),
        { status: 200 }
      )
    );
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(telegramRequest({ origin: "https://drivergo.uz" }));
    const json = await response.json();

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8090/api/v1/auth/telegram/webapp",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ init_data: "query_id=1&user=%7B%7D&auth_date=1&hash=ff" }),
      })
    );
    expect(response.status).toBe(200);
    expect(json).toEqual({ data: { ok: true, must_change_password: true } });
    expect(JSON.stringify(json)).not.toContain("abc.def");

    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("abc.def");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("xyz.123");
    const [atExpiry, rtExpiry, ...issued] = response.headers.getSetCookie();
    // A lax session the Android WebView jar may already hold is expired first.
    expect(atExpiry).toMatch(/^at=; Path=\/; Max-Age=0; HttpOnly; SameSite=Lax/);
    expect(rtExpiry).toMatch(/^rt=; Path=\/; Max-Age=0; HttpOnly; SameSite=Lax/);
    const all = issued.join("\n").toLowerCase();
    expect(issued).toHaveLength(3);
    expect(all).toContain("partitioned");
    expect(all).toContain(`${TG_MODE_COOKIE}=1`);
    expect(all).not.toContain("samesite=lax");
  });

  it("passes need_phone through without setting cookies", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ data: { need_phone: true, first_name: "Ali" } }), { status: 200 })
        )
    );

    const response = await POST(telegramRequest());

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ data: { need_phone: true, first_name: "Ali" } });
    expect(response.headers.getSetCookie()).toEqual([]);
  });

  it("passes a backend 401 through without setting cookies", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ error: { code: "invalid_init_data", message: "bad" } }), { status: 401 })
        )
    );

    const response = await POST(telegramRequest());

    expect(response.status).toBe(401);
    expect((await response.json()).error.code).toBe("invalid_init_data");
    expect(response.headers.getSetCookie()).toEqual([]);
  });

  it("returns 502 when the backend is unreachable or answers without tokens", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("down")));
    const down = await POST(telegramRequest());
    expect(down.status).toBe(502);
    expect((await down.json()).error.code).toBe("network_error");

    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: {} }), { status: 200 })));
    const malformed = await POST(telegramRequest());
    expect(malformed.status).toBe(502);
    expect(malformed.headers.getSetCookie()).toEqual([]);
  });

  it("refuses a foreign Origin with 403 before calling the backend", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(telegramRequest({ origin: "https://evil.example" }));

    expect(response.status).toBe(403);
    expect((await response.json()).error.code).toBe("cross_site");
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
