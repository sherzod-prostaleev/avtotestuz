import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { adminProxy, adminStreamProxy } from "./admin-proxy";
import { ADMIN_AUTH_COOKIE, ADMIN_REFRESH_COOKIE } from "./admin-auth-cookies";
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
