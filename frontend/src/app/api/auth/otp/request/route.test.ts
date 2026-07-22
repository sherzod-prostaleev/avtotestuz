import { createHmac } from "node:crypto";
import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { POST } from "./route";

const assertionSecret = "otp-client-ip-test-secret-32-bytes!";

beforeEach(() => {
  vi.stubEnv("NODE_ENV", "test");
  vi.stubEnv("CLIENT_IP_ASSERTION_SECRET", "");
  vi.stubEnv("TRUSTED_PROXY_HOPS", "");
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  vi.useRealTimers();
});

describe("POST /api/auth/otp/request", () => {
  it("forwards the request body to the backend and returns its response verbatim", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ data: { channel: "sandbox", debug_code: "123456" } }), { status: 200 })
      );
    vi.stubGlobal("fetch", fetchMock);

    const request = new Request("http://localhost/api/auth/otp/request", {
      method: "POST",
      body: JSON.stringify({ phone: "901112233" }),
    });
    const response = await POST(request);
    const json = await response.json();

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8090/api/v1/auth/otp/request",
      expect.objectContaining({ method: "POST" })
    );
    expect(response.status).toBe(200);
    expect(json).toEqual({ data: { channel: "sandbox", debug_code: "123456" } });
  });

  it("passes through a rate_limited error with its status code", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ error: { code: "rate_limited", message: "too many requests" } }), {
            status: 429,
          })
        )
    );

    const request = new Request("http://localhost/api/auth/otp/request", {
      method: "POST",
      body: JSON.stringify({ phone: "901112233" }),
    });
    const response = await POST(request);

    expect(response.status).toBe(429);
    expect((await response.json()).error.code).toBe("rate_limited");
  });

  it("returns a stable 502 instead of throwing when the backend is unavailable", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("connect ECONNREFUSED")));

    const request = new Request("http://localhost/api/auth/otp/request", {
      method: "POST",
      body: JSON.stringify({ phone: "901112233" }),
    });
    const response = await POST(request);

    expect(response.status).toBe(502);
    expect((await response.json()).error.code).toBe("network_error");
  });

  it("selects client IPs from the configured trusted proxy hop and signs distinct buckets", async () => {
    vi.stubEnv("CLIENT_IP_ASSERTION_SECRET", assertionSecret);
    vi.stubEnv("TRUSTED_PROXY_HOPS", "1");
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-07-22T12:00:00Z"));
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: { channel: "sandbox" } }), { status: 200 })
    );
    vi.stubGlobal("fetch", fetchMock);

    const postFrom = (forwardedFor: string) =>
      POST(
        new Request("http://localhost/api/auth/otp/request", {
          method: "POST",
          headers: { "X-Forwarded-For": forwardedFor },
          body: JSON.stringify({ phone: "901112233" }),
        })
      );

    await postFrom("203.0.113.250, 198.51.100.10");
    await postFrom("203.0.113.250, 198.51.100.11");

    const firstHeaders = new Headers(fetchMock.mock.calls[0][1]?.headers);
    const secondHeaders = new Headers(fetchMock.mock.calls[1][1]?.headers);
    expect(firstHeaders.get("X-Avtotest-Client-IP")).toBe("198.51.100.10");
    expect(secondHeaders.get("X-Avtotest-Client-IP")).toBe("198.51.100.11");
    expect(firstHeaders.get("X-Avtotest-Client-IP-Signature")).not.toBe(
      secondHeaders.get("X-Avtotest-Client-IP-Signature")
    );

    const timestamp = firstHeaders.get("X-Avtotest-Client-IP-Timestamp");
    const payload = ["v1", timestamp, "198.51.100.10", "POST", "/api/v1/auth/otp/request"].join("\n");
    expect(firstHeaders.get("X-Avtotest-Client-IP-Signature")).toBe(
      createHmac("sha256", assertionSecret).update(payload).digest("base64url")
    );
  });

  it("fails closed when an enabled trusted proxy chain is missing", async () => {
    vi.stubEnv("CLIENT_IP_ASSERTION_SECRET", assertionSecret);
    vi.stubEnv("TRUSTED_PROXY_HOPS", "1");
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(
      new Request("http://localhost/api/auth/otp/request", {
        method: "POST",
        body: JSON.stringify({ phone: "901112233" }),
      })
    );

    expect(response.status).toBe(502);
    expect((await response.json()).error.code).toBe("network_error");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("fails closed in production when assertion configuration is absent", async () => {
    vi.stubEnv("NODE_ENV", "production");
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(
      new Request("http://localhost/api/auth/otp/request", {
        method: "POST",
        body: JSON.stringify({ phone: "901112233" }),
      })
    );

    expect(response.status).toBe(502);
    expect((await response.json()).error.code).toBe("network_error");
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
