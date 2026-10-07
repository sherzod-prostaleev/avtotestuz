import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE, TG_MODE_COOKIE } from "@/lib/auth-cookies";

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

  it("returns 502 without clearing cookies on a transient backend failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network down")));

    const response = await POST(requestWithCookie("rt=still-usable"));
    const json = await response.json();

    expect(response.status).toBe(502);
    expect(json.error.code).toBe("network_error");
    expect(response.cookies.get(AUTH_COOKIE)).toBeUndefined();
    expect(response.cookies.get(REFRESH_COOKIE)).toBeUndefined();
  });

  it("re-issues Partitioned cookies when the request carries the Telegram marker", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ data: { access_token: "new-at", refresh_token: "new-rt" } }), { status: 200 })
        )
    );

    const response = await POST(requestWithCookie(`rt=old-rt; ${TG_MODE_COOKIE}=1`));

    expect(response.status).toBe(200);
    const cookies = response.headers.getSetCookie();
    expect(cookies).toHaveLength(3);
    for (const c of cookies) expect(c.toLowerCase()).toContain("partitioned");
  });

  it("clears with Partitioned when a Telegram refresh is rejected", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(new Response(JSON.stringify({ error: { code: "refresh_reused" } }), { status: 401 }))
    );

    const response = await POST(requestWithCookie(`rt=stolen-rt; ${TG_MODE_COOKIE}=1`));

    expect(response.status).toBe(401);
    const cookies = response.headers.getSetCookie();
    expect(cookies).toHaveLength(3);
    for (const c of cookies) {
      expect(c.toLowerCase()).toContain("partitioned");
      expect(c.toLowerCase()).toContain("max-age=0");
    }
  });

  it("keeps site refreshes lax", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ data: { access_token: "new-at", refresh_token: "new-rt" } }), { status: 200 })
        )
    );

    const response = await POST(requestWithCookie("rt=old-rt"));

    const all = response.headers.getSetCookie().join("\n").toLowerCase();
    expect(all).toContain("samesite=lax");
    expect(all).not.toContain("partitioned");
  });

  it("refuses a foreign Origin with 403 before calling the backend", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(
      new Request("http://localhost/api/auth/refresh", {
        method: "POST",
        headers: { host: "drivergo.uz", origin: "https://evil.example", Cookie: "rt=old-rt" },
      })
    );

    expect(response.status).toBe(403);
    expect(response.headers.getSetCookie()).toEqual([]);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
