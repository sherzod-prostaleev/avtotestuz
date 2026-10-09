import { afterEach, describe, expect, it, vi } from "vitest";
import { POST as start } from "./start/route";
import { GET as status } from "./status/route";
import { POST as complete } from "./complete/route";
import { POST as phone } from "../telegram/phone/route";
import { AUTH_COOKIE, REFRESH_COOKIE, TG_MODE_COOKIE } from "@/lib/auth-cookies";
import { TELEGRAM_LOGIN_COOKIE } from "@/lib/telegram-login-cookie";

const TOKEN = "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789_-abcde";
const UA = "Mozilla/5.0 (Linux; Android 14) Chrome/128.0.0.0 Mobile Safari/537.36";

afterEach(() => {
  vi.unstubAllGlobals();
});

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status });
}

describe("POST /api/auth/telegram-login/start", () => {
  it("forwards the browser's User-Agent, keeps the secret in an HttpOnly cookie and out of the body", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      json({ data: { bot_url: `https://t.me/DriverGouzBot?start=login_${TOKEN}`, token: TOKEN, browser_secret: "s3cr3t-value", expires_in_sec: 300 } })
    );
    vi.stubGlobal("fetch", fetchMock);
    const res = await start(
      new Request("http://localhost/api/auth/telegram-login/start", {
        method: "POST",
        headers: { "user-agent": UA, host: "drivergo.uz", origin: "https://drivergo.uz" },
      })
    );
    const body = await res.json();
    expect(res.status).toBe(200);
    expect(body).toEqual({ data: { bot_url: `https://t.me/DriverGouzBot?start=login_${TOKEN}`, token: TOKEN, expires_in_sec: 300 } });
    expect(JSON.stringify(body)).not.toContain("s3cr3t-value");
    const cookie = res.cookies.get(TELEGRAM_LOGIN_COOKIE);
    expect(cookie?.value).toBe("s3cr3t-value");
    expect(cookie?.httpOnly).toBe(true);
    expect(cookie?.sameSite).toBe("lax");
    expect(cookie?.path).toBe("/api/auth/telegram-login");
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain("/api/v1/auth/telegram-login/start");
    expect(JSON.parse(init.body as string)).toEqual({ user_agent: UA });
  });

  it("refuses a cross-site start", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const res = await start(
      new Request("http://localhost/api/auth/telegram-login/start", {
        method: "POST",
        headers: { host: "drivergo.uz", origin: "https://evil.example" },
      })
    );
    expect(res.status).toBe(403);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("passes the bot-unconfigured error through and sets no cookie", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ error: { code: "telegram_bot_unconfigured" } }, 503)));
    const res = await start(new Request("http://localhost/api/auth/telegram-login/start", { method: "POST" }));
    expect(res.status).toBe(503);
    expect(res.cookies.get(TELEGRAM_LOGIN_COOKIE)).toBeUndefined();
  });
});

describe("GET /api/auth/telegram-login/status", () => {
  it("sends token + cookie secret to the backend in a POST body", async () => {
    const fetchMock = vi.fn().mockResolvedValue(json({ data: { state: "approved" } }));
    vi.stubGlobal("fetch", fetchMock);
    const res = await status(
      new Request(`http://localhost/api/auth/telegram-login/status?token=${TOKEN}`, {
        headers: { cookie: `${TELEGRAM_LOGIN_COOKIE}=sec` },
      })
    );
    expect(await res.json()).toEqual({ data: { state: "approved" } });
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ token: TOKEN, browser_secret: "sec" });
  });

  it("answers invalid without a cookie or with a malformed token, never calling the backend", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    for (const req of [
      new Request(`http://localhost/api/auth/telegram-login/status?token=${TOKEN}`),
      new Request(`http://localhost/api/auth/telegram-login/status?token=bad%20token`, {
        headers: { cookie: `${TELEGRAM_LOGIN_COOKIE}=sec` },
      }),
    ]) {
      const res = await status(req);
      expect(await res.json()).toEqual({ data: { state: "invalid" } });
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("POST /api/auth/telegram-login/complete", () => {
  it("sets the website's lax session cookies, drops the login cookie, never echoes tokens", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(json({ data: { access_token: "abc.def", refresh_token: "xyz.123", created: true } }))
    );
    const res = await complete(
      new Request("http://localhost/api/auth/telegram-login/complete", {
        method: "POST",
        headers: { cookie: `${TELEGRAM_LOGIN_COOKIE}=sec`, host: "drivergo.uz", origin: "https://drivergo.uz" },
        body: JSON.stringify({ token: TOKEN }),
      })
    );
    const body = await res.json();
    expect(body).toEqual({ data: { ok: true, must_change_password: false, created: true } });
    expect(JSON.stringify(body)).not.toContain("abc.def");
    expect(res.cookies.get(AUTH_COOKIE)?.value).toBe("abc.def");
    expect(res.cookies.get(AUTH_COOKIE)?.sameSite).toBe("lax");
    expect(res.cookies.get(REFRESH_COOKIE)?.value).toBe("xyz.123");
    expect(res.cookies.get(TG_MODE_COOKIE)).toBeUndefined();
    expect(res.cookies.get(TELEGRAM_LOGIN_COOKIE)?.value).toBe("");
  });

  it("without the browser cookie it refuses before reaching the backend", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const res = await complete(
      new Request("http://localhost/api/auth/telegram-login/complete", { method: "POST", body: JSON.stringify({ token: TOKEN }) })
    );
    expect(res.status).toBe(400);
    expect((await res.json()).error.code).toBe("invalid_login_request");
    expect(fetchMock).not.toHaveBeenCalled();
    expect(res.cookies.get(AUTH_COOKIE)).toBeUndefined();
  });

  it("keeps the login cookie while not yet approved, drops it on a final error", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ error: { code: "login_not_approved" } }, 409)));
    const req = () =>
      new Request("http://localhost/api/auth/telegram-login/complete", {
        method: "POST",
        headers: { cookie: `${TELEGRAM_LOGIN_COOKIE}=sec` },
        body: JSON.stringify({ token: TOKEN }),
      });
    const early = await complete(req());
    expect(early.status).toBe(409);
    expect(early.cookies.get(TELEGRAM_LOGIN_COOKIE)).toBeUndefined();

    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ error: { code: "invalid_login_request" } }, 400)));
    const final = await complete(req());
    expect(final.status).toBe(400);
    expect(final.cookies.get(TELEGRAM_LOGIN_COOKIE)?.value).toBe("");
    expect(final.cookies.get(AUTH_COOKIE)).toBeUndefined();
  });
});

describe("POST /api/auth/telegram/phone", () => {
  it("issues Telegram-mode cookies for the Mini App phone sign-in", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      json({ data: { access_token: "abc.def", refresh_token: "xyz.123", telegram_linked: true, created: true } })
    );
    vi.stubGlobal("fetch", fetchMock);
    const res = await phone(
      new Request("http://localhost/api/auth/telegram/phone", {
        method: "POST",
        headers: { host: "drivergo.uz", origin: "https://drivergo.uz" },
        body: JSON.stringify({ init_data: "a=1&hash=ff", contact: "contact=%7B%7D&hash=ff" }),
      })
    );
    expect(await res.json()).toEqual({ data: { ok: true, must_change_password: false, created: true } });
    expect(res.cookies.get(AUTH_COOKIE)?.sameSite).toBe("none");
    expect(res.cookies.get(TG_MODE_COOKIE)?.value).toBe("1");
    const [url] = fetchMock.mock.calls[0] as [string];
    expect(url).toContain("/api/v1/auth/telegram/webapp/phone");
  });

  it("passes invalid_phone through without cookies", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ error: { code: "invalid_phone" } }, 400)));
    const res = await phone(new Request("http://localhost/api/auth/telegram/phone", { method: "POST", body: "{}" }));
    expect(res.status).toBe(400);
    expect(res.cookies.get(AUTH_COOKIE)).toBeUndefined();
  });
});
