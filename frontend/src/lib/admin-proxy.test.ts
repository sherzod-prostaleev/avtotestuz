import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { adminProxy, adminStreamProxy } from "./admin-proxy";
import {
  ADMIN_AUTH_COOKIE,
  ADMIN_REFRESH_COOKIE,
  ADMIN_TOTP_ENROLL_COOKIE,
} from "./admin-auth-cookies";
import { resetRefreshLockForTests } from "./refresh-lock";

function adminRequest(): Request {
  return new Request("http://localhost/api/admin/test", {
    headers: { Cookie: "aat=expired-at; art=old-rt" },
  });
}

beforeEach(() => {
  resetRefreshLockForTests();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("adminProxy refresh rotation", () => {
  it("stores both rotated cookies for JSON responses", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response("{}", { status: 401 }))
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ data: { access_token: "fresh-at", refresh_token: "fresh-rt" } }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: { ok: true } }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await adminProxy(adminRequest(), "/me", { method: "GET" });

    expect(response.status).toBe(200);
    expect(response.cookies.get(ADMIN_AUTH_COOKIE)?.value).toBe("fresh-at");
    expect(response.cookies.get(ADMIN_REFRESH_COOKIE)?.value).toBe("fresh-rt");
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "http://localhost:8090/admin/v1/me",
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: "Bearer fresh-at" }),
      }),
    );
  });

  it("preserves a CSV body and rotated cookies after refresh", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(new Response("{}", { status: 401 }))
        .mockResolvedValueOnce(
          new Response(
            JSON.stringify({ data: { access_token: "fresh-at", refresh_token: "fresh-rt" } }),
            { status: 200 },
          ),
        )
        .mockResolvedValueOnce(
          new Response("name,score\nAli,10\n", {
            status: 200,
            headers: {
              "Content-Type": "text/csv; charset=utf-8",
              "Content-Disposition": 'attachment; filename="report.csv"',
            },
          }),
        ),
    );

    const response = await adminProxy(adminRequest(), "/report.csv", { method: "GET" });

    expect(await response.text()).toBe("name,score\nAli,10\n");
    expect(response.headers.get("content-type")).toContain("text/csv");
    expect(response.headers.get("content-disposition")).toContain("report.csv");
    expect(response.cookies.get(ADMIN_REFRESH_COOKIE)?.value).toBe("fresh-rt");
  });

  it("rotates cookies while streaming SSE without buffering it as JSON", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(new Response("{}", { status: 401 }))
        .mockResolvedValueOnce(
          new Response(
            JSON.stringify({ data: { access_token: "fresh-at", refresh_token: "fresh-rt" } }),
            { status: 200 },
          ),
        )
        .mockResolvedValueOnce(
          new Response("event: ready\ndata: {}\n\n", {
            status: 200,
            headers: { "Content-Type": "text/event-stream" },
          }),
        ),
    );

    const response = await adminStreamProxy(adminRequest(), "/monitoring/stream");

    expect(response.headers.get("content-type")).toContain("text/event-stream");
    expect(await response.text()).toBe("event: ready\ndata: {}\n\n");
    expect(response.cookies.get(ADMIN_AUTH_COOKIE)?.value).toBe("fresh-at");
    expect(response.cookies.get(ADMIN_REFRESH_COOKIE)?.value).toBe("fresh-rt");
  });

  it("does not clear the old cookies on an upstream refresh failure", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(new Response("{}", { status: 401 }))
        .mockResolvedValueOnce(new Response("down", { status: 503 })),
    );

    const response = await adminProxy(adminRequest(), "/me", { method: "GET" });

    expect(response.status).toBe(502);
    expect(response.cookies.get(ADMIN_AUTH_COOKIE)).toBeUndefined();
    expect(response.cookies.get(ADMIN_REFRESH_COOKIE)).toBeUndefined();
  });
});

function enrollRequest(cookie: string): Request {
  return new Request("http://localhost/api/admin/security/totp/enroll", {
    method: "POST",
    headers: { Cookie: cookie },
  });
}

function authHeaderOf(call: unknown[]): string | undefined {
  const init = call[1] as RequestInit | undefined;
  return (init?.headers as Record<string, string> | undefined)?.Authorization;
}

describe("adminProxy TOTP enrollment credential", () => {
  it("uses the enrollment cookie for the two enrollment paths when there is no session", async () => {
    // A fresh Response per call: a Response body can only be read once.
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ data: { secret: "S3CR3T", otpauth_url: "otpauth://x" } }), {
        status: 200,
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    for (const path of ["/security/totp/enroll", "/security/totp/confirm"]) {
      fetchMock.mockClear();
      const response = await adminProxy(
        enrollRequest("ate=enroll-token"),
        path,
        { method: "POST", body: "{}" },
        { allowEnrollmentToken: true },
      );

      expect(response.status).toBe(200);
      expect(authHeaderOf(fetchMock.mock.calls[0])).toBe("Bearer enroll-token");
      // No session was involved, so no session cookie may be written.
      expect(response.cookies.get(ADMIN_AUTH_COOKIE)).toBeUndefined();
      expect(response.cookies.get(ADMIN_REFRESH_COOKIE)).toBeUndefined();
    }
  });

  it("never sends the enrollment token to any other path, even if asked to", async () => {
    const fetchMock = vi.fn(async () =>
      new Response(JSON.stringify({ data: {} }), { status: 200 }),
    );
    vi.stubGlobal("fetch", fetchMock);

    for (const path of ["/me", "/security/totp/disable", "/users"]) {
      fetchMock.mockClear();
      await adminProxy(
        enrollRequest("ate=enroll-token"),
        path,
        { method: "POST", body: "{}" },
        { allowEnrollmentToken: true },
      );

      expect(authHeaderOf(fetchMock.mock.calls[0])).toBe("");
    }
  });

  it("ignores the enrollment cookie unless the route opts in", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ data: {} }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await adminProxy(enrollRequest("ate=enroll-token"), "/security/totp/enroll", {
      method: "POST",
      body: "{}",
    });

    expect(authHeaderOf(fetchMock.mock.calls[0])).toBe("");
  });

  it("prefers a live admin session over the enrollment cookie", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ data: { secret: "S" } }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    const response = await adminProxy(
      enrollRequest("aat=live-at; ate=stale-enroll"),
      "/security/totp/enroll",
      { method: "POST", body: "{}" },
      { allowEnrollmentToken: true },
    );

    expect(authHeaderOf(fetchMock.mock.calls[0])).toBe("Bearer live-at");
    expect(response.cookies.get(ADMIN_TOTP_ENROLL_COOKIE)).toBeUndefined();
  });

  it("refreshes a session whose access token expired rather than using the enrollment cookie", async () => {
    // `aat` lives 15 minutes and `art` 30 days, so "no aat" is routinely a
    // healthy, refreshable session — it must not be treated as "no session".
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response("{}", { status: 401 }))
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ data: { access_token: "fresh-at", refresh_token: "fresh-rt" } }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: { secret: "S" } }), { status: 200 }),
      );
    vi.stubGlobal("fetch", fetchMock);

    const response = await adminProxy(
      enrollRequest("art=healthy-rt; ate=stale-enroll"),
      "/security/totp/enroll",
      { method: "POST", body: "{}" },
      { allowEnrollmentToken: true },
    );

    expect(response.status).toBe(200);
    expect(authHeaderOf(fetchMock.mock.calls[2])).toBe("Bearer fresh-at");
    expect(response.cookies.get(ADMIN_AUTH_COOKIE)?.value).toBe("fresh-at");
  });

  it("does not clear admin session cookies when an enrollment attempt is rejected", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ error: { code: "unauthorized" } }), { status: 401 }),
      );
    vi.stubGlobal("fetch", fetchMock);

    const response = await adminProxy(
      enrollRequest("ate=expired-enroll"),
      "/security/totp/confirm",
      { method: "POST", body: '{"code":"123456"}' },
      { allowEnrollmentToken: true },
    );

    expect(response.status).toBe(401);
    // Exactly one upstream call: no refresh is attempted for this credential.
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(response.cookies.get(ADMIN_AUTH_COOKIE)).toBeUndefined();
    expect(response.cookies.get(ADMIN_REFRESH_COOKIE)).toBeUndefined();
    // The spent enrollment token itself is dropped.
    expect(response.cookies.get(ADMIN_TOTP_ENROLL_COOKIE)?.value).toBe("");
    expect(response.cookies.get(ADMIN_TOTP_ENROLL_COOKIE)?.maxAge).toBe(0);
  });

  it("forwards an invalid-code 400 without touching any cookie", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ error: { code: "invalid_totp" } }), { status: 400 }),
        ),
    );

    const response = await adminProxy(
      enrollRequest("ate=enroll-token"),
      "/security/totp/confirm",
      { method: "POST", body: '{"code":"000000"}' },
      { allowEnrollmentToken: true },
    );

    expect(response.status).toBe(400);
    expect(response.cookies.get(ADMIN_TOTP_ENROLL_COOKIE)).toBeUndefined();
    expect(response.cookies.get(ADMIN_AUTH_COOKIE)).toBeUndefined();
  });
});
