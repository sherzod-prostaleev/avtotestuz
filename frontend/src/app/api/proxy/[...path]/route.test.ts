import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { GET, POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";
import { resetRefreshLockForTests } from "@/lib/refresh-lock";

beforeEach(() => {
  resetRefreshLockForTests();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function requestWithCookies(cookieHeader: string, init: RequestInit = {}): Request {
  const headers = new Headers(init.headers);
  headers.set("Cookie", cookieHeader);
  return new Request("http://localhost/api/proxy/x", { ...init, headers });
}

describe("proxy route", () => {
  it("returns 401 with no backend call when there is no access token cookie", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(requestWithCookies(""), { params: { path: ["me"] } });

    expect(response.status).toBe(401);
    expect((await response.json()).error.code).toBe("unauthorized");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("forwards a GET with the Bearer token and returns the backend response verbatim", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ data: { id: "profile-1" } }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(requestWithCookies("at=good-token"), { params: { path: ["me"] } });
    const json = await response.json();

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8090/api/v1/me",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({ Authorization: "Bearer good-token" }),
      })
    );
    expect(json).toEqual({ data: { id: "profile-1" } });
  });

  it("on a 401, refreshes once and retries the same request with the new token", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "unauthorized" } }), { status: 401 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { access_token: "fresh-at", refresh_token: "fresh-rt" } }), {
          status: 200,
        })
      )
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { id: "profile-1" } }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(requestWithCookies("at=expired-token; rt=valid-rt"), { params: { path: ["me"] } });
    const json = await response.json();

    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "http://localhost:8090/api/v1/me",
      expect.objectContaining({ headers: expect.objectContaining({ Authorization: "Bearer fresh-at" }) })
    );
    expect(json).toEqual({ data: { id: "profile-1" } });
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("fresh-at");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("fresh-rt");
  });

  it("clears cookies and returns 401 when the refresh itself fails", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "unauthorized" } }), { status: 401 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "refresh_reused" } }), { status: 401 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(requestWithCookies("at=expired-token; rt=stolen-rt"), { params: { path: ["me"] } });
    const json = await response.json();

    expect(response.status).toBe(401);
    expect(json.error.code).toBe("unauthorized");
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("keeps rotated cookies when retry still 401s after a successful refresh", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "unauthorized" } }), { status: 401 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { access_token: "fresh-at", refresh_token: "fresh-rt" } }), {
          status: 200,
        })
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: { code: "unauthorized", message: "endpoint denied" } }), {
          status: 401,
        })
      );
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(requestWithCookies("at=expired-token; rt=valid-rt"), { params: { path: ["me"] } });

    expect(response.status).toBe(401);
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("fresh-at");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("fresh-rt");
  });

  it("returns 502 without clearing cookies when refresh upstream is 5xx", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "unauthorized" } }), { status: 401 }))
      .mockResolvedValueOnce(new Response("upstream down", { status: 503 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(requestWithCookies("at=expired-token; rt=valid-rt"), { params: { path: ["me"] } });

    expect(response.status).toBe(502);
    expect((await response.json()).error.code).toBe("network_error");
    expect(response.cookies.get(AUTH_COOKIE)).toBeUndefined();
    expect(response.cookies.get(REFRESH_COOKIE)).toBeUndefined();
  });

  it("forwards a POST body correctly, reusing the exact same body across the retry", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "unauthorized" } }), { status: 401 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { access_token: "fresh-at", refresh_token: "fresh-rt" } }), {
          status: 200,
        })
      )
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { ok: true } }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const body = JSON.stringify({ question_id: "q1", answer_id: "a1" });
    await POST(requestWithCookies("at=expired-token; rt=valid-rt", { method: "POST", body }), {
      params: { path: ["sessions", "abc", "answers"] },
    });

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "http://localhost:8090/api/v1/sessions/abc/answers",
      expect.objectContaining({ method: "POST", body })
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "http://localhost:8090/api/v1/sessions/abc/answers",
      expect.objectContaining({ method: "POST", body })
    );
  });

  it("clears cookies on 403 account_blocked so a banned learner cannot keep calling", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ error: { code: "account_blocked", message: "account is blocked" } }), {
          status: 403,
        })
      );
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(requestWithCookies("at=live-token; rt=live-rt"), { params: { path: ["me"] } });
    const json = await response.json();

    expect(response.status).toBe(403);
    expect(json.error.code).toBe("account_blocked");
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("");
  });

  it("returns a stable 502 and preserves auth cookies when the backend is unavailable", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network down")));

    const response = await GET(requestWithCookies("at=good-token; rt=good-refresh"), {
      params: { path: ["me"] },
    });

    expect(response.status).toBe(502);
    expect((await response.json()).error.code).toBe("network_error");
    expect(response.cookies.get(AUTH_COOKIE)).toBeUndefined();
    expect(response.cookies.get(REFRESH_COOKIE)).toBeUndefined();
  });

  it("keeps newly rotated cookies when the retried backend request has a network failure", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: { code: "unauthorized" } }), { status: 401 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { access_token: "fresh-at", refresh_token: "fresh-rt" } }), {
          status: 200,
        })
      )
      .mockRejectedValueOnce(new Error("backend disappeared"));
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(requestWithCookies("at=expired; rt=old-rt"), {
      params: { path: ["me"] },
    });

    expect(response.status).toBe(502);
    expect(response.cookies.get(AUTH_COOKIE)?.value).toBe("fresh-at");
    expect(response.cookies.get(REFRESH_COOKIE)?.value).toBe("fresh-rt");
  });

  it("rejects traversal-like route segments before contacting the backend", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(requestWithCookies("at=good-token"), {
      params: { path: ["..", "auth", "logout"] },
    });

    expect(response.status).toBe(400);
    expect((await response.json()).error.code).toBe("invalid_path");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("returns 502 for a malformed successful backend response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("not-json", { status: 200 })));

    const response = await GET(requestWithCookies("at=good-token"), {
      params: { path: ["me"] },
    });

    expect(response.status).toBe(502);
    expect((await response.json()).error.code).toBe("network_error");
  });

  it("forwards public variants detail without auth cookies", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ data: { number: 1, questions: [] } }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await GET(new Request("http://localhost/api/proxy/variants/1?locale=uz-Latn"), {
      params: { path: ["variants", "1"] },
    });

    expect(response.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8090/api/v1/variants/1?locale=uz-Latn",
      expect.objectContaining({ method: "GET" })
    );
  });
});
