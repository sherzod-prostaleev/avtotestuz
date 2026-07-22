import { describe, it, expect, vi, afterEach } from "vitest";
import { POST } from "./route";

afterEach(() => {
  vi.unstubAllGlobals();
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
});
