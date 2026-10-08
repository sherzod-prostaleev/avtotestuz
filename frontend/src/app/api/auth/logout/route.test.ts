import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE, TG_MODE_COOKIE } from "@/lib/auth-cookies";

afterEach(() => {
  vi.unstubAllGlobals();
});

function requestWithCookie(cookieHeader?: string): Request {
  const headers: Record<string, string> = cookieHeader ? { Cookie: cookieHeader } : {};
  return new Request("http://localhost/api/auth/logout", { method: "POST", headers });
}

describe("POST /api/auth/logout", () => {
  it("clears cookies when the backend call succeeds", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(requestWithCookie("rt=some-rt; at=some-at"));

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8090/api/v1/auth/logout",
      expect.objectContaining({ method: "POST" })
    );
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("");
  });

  it("still clears cookies when the backend call throws (network error)", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network down")));

    const response = await POST(requestWithCookie("rt=some-rt; at=some-at"));

    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("");
  });

  it("clears cookies even when there was no refresh token to send", async () => {
    const response = await POST(requestWithCookie(undefined));
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("");
  });

  it("clears with Partitioned, marker included, for a Telegram session", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 })));

    const response = await POST(requestWithCookie(`rt=some-rt; at=some-at; ${TG_MODE_COOKIE}=1`));

    const partitioned = response.headers.getSetCookie().filter((c) => c.toLowerCase().includes("partitioned"));
    expect(partitioned.map((c) => c.split("=")[0]).sort()).toEqual(["at", "rt", TG_MODE_COOKIE].sort());
    for (const c of partitioned) expect(c.toLowerCase()).toContain("max-age=0");
  });

  // Telegram's Android WebView can hold the website's unpartitioned lax pair
  // beside the partitioned one; logout must end both, not just the one the
  // Telegram mode cookie points at.
  it("in Telegram mode also expires the lax site pair, first, and revokes every refresh token sent", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(
      requestWithCookie(`rt=tg-rt; at=tg-at; rt=site-rt; at=site-at; ${TG_MODE_COOKIE}=1`)
    );

    const cookies = response.headers.getSetCookie();
    const lax = cookies.filter((c) => !c.toLowerCase().includes("partitioned"));
    expect(lax.map((c) => c.split("=")[0]).sort()).toEqual(["at", "rt"]);
    for (const c of lax) {
      expect(c).toContain("Max-Age=0");
      expect(c).toContain("SameSite=Lax");
    }
    // WebKit ignores Partitioned: the lax expiries must not come last, or they
    // would be the final word on a cookie the browser sees as one.
    const firstPartitioned = cookies.findIndex((c) => c.toLowerCase().includes("partitioned"));
    expect(cookies.slice(0, firstPartitioned)).toEqual(lax);

    const sent = fetchMock.mock.calls.map(([, init]) => JSON.parse((init as RequestInit).body as string).refresh_token);
    expect(sent.sort()).toEqual(["site-rt", "tg-rt"]);
  });

  it("site-mode logout is unchanged: one lax pair, no Partitioned, one revoke", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(requestWithCookie("rt=some-rt; at=some-at"));

    const cookies = response.headers.getSetCookie();
    expect(cookies.map((c) => c.split("=")[0]).sort()).toEqual(["at", "rt"]);
    for (const c of cookies) expect(c.toLowerCase()).not.toContain("partitioned");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("refuses a foreign Origin with 403 and leaves the session alone", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(
      new Request("http://localhost/api/auth/logout", {
        method: "POST",
        headers: { host: "drivergo.uz", origin: "https://evil.example", Cookie: "rt=some-rt" },
      })
    );

    expect(response.status).toBe(403);
    expect(response.headers.getSetCookie()).toEqual([]);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
