import { describe, it, expect, vi, afterEach } from "vitest";
import { GET, POST } from "./route";
import { AUTH_COOKIE, REFRESH_COOKIE } from "@/lib/auth-cookies";

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
});
