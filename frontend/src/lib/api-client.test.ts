import { describe, it, expect, vi, beforeEach } from "vitest";
import { apiGet, apiPost, apiPatch, apiDelete, ApiError } from "./api-client";

describe("api-client", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("apiGet returns data on success", async () => {
    const mockResponse = { data: { readiness_pct: 85 } };
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => mockResponse,
    } as Response);

    const data = await apiGet<{ readiness_pct: number }>("me/stats");
    expect(data.readiness_pct).toBe(85);
    expect(global.fetch).toHaveBeenCalledWith("/api/proxy/me/stats", expect.objectContaining({ method: "GET" }));
  });

  it("apiGet forwards an abort signal", async () => {
    const controller = new AbortController();
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ data: { ok: true } }),
    } as Response);
    await apiGet("me", { signal: controller.signal });
    expect(global.fetch).toHaveBeenCalledWith(
      "/api/proxy/me",
      expect.objectContaining({ method: "GET", signal: controller.signal }),
    );
  });

  it("apiPost sends JSON payload and returns response", async () => {
    const mockResponse = { data: { id: "sess-123" } };
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => mockResponse,
    } as Response);

    const data = await apiPost<{ id: string }>("sessions", { mode: "exam" });
    expect(data.id).toBe("sess-123");
    expect(global.fetch).toHaveBeenCalledWith(
      "/api/proxy/sessions",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ mode: "exam" }),
      })
    );
  });

  it("throws ApiError on non-ok status with error payload", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 402,
      json: async () => ({ error: { code: "vip_required", message: "VIP subscription required" } }),
    } as Response);

    await expect(apiGet("me/mistakes")).rejects.toThrow(ApiError);
    try {
      await apiGet("me/mistakes");
    } catch (err: unknown) {
      const apiErr = err as ApiError;
      expect(apiErr.code).toBe("vip_required");
      expect(apiErr.status).toBe(402);
      expect(apiErr.message).toBe("VIP subscription required");
    }
  });

  // A 502 does not come from the API -- it comes from nginx or Cloudflare with
  // an HTML body, so there is no error envelope to read a code out of and the
  // client has to synthesise one. It used to synthesise `upstream_unreachable`,
  // which nothing anywhere consumed: every screen that handles a broken
  // transport branches on `network_error` (the code the BFF returns when it
  // cannot reach Go), so a 502 fell through to the generic "something went
  // wrong" instead of "your session is saved, check your connection and retry".
  // Two codes for one user-visible condition is what let them drift apart, so
  // the transport failure now has exactly one. The HTTP status is still on the
  // error for anyone who needs to tell the two apart.
  it.each([
    [502, "nginx returns HTML on a bad gateway"],
    [504, "gateway timeout, same class of failure"],
  ])("maps a bodyless %i to network_error, not a code nobody handles", async (status) => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status,
      json: async () => {
        throw new SyntaxError("Unexpected token < in JSON at position 0");
      },
    } as unknown as Response);

    try {
      await apiPost("sessions/abc/answers", { answer_id: "x" });
      throw new Error("expected the request to reject");
    } catch (err: unknown) {
      const apiErr = err as ApiError;
      expect(apiErr).toBeInstanceOf(ApiError);
      expect(apiErr.code).toBe("network_error");
      expect(apiErr.status).toBe(status);
    }
  });

  // 503 stays distinct: it is the classroom station agent failing closed while
  // it has no token, which the kiosk screen tells apart from a dead upstream.
  it("keeps 503 as station_offline", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 503,
      json: async () => {
        throw new SyntaxError("not json");
      },
    } as unknown as Response);

    try {
      await apiGet("me");
      throw new Error("expected the request to reject");
    } catch (err: unknown) {
      expect((err as ApiError).code).toBe("station_offline");
    }
  });

  it("apiPatch and apiDelete work as expected", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ data: { success: true } }),
    } as Response);

    const patchRes = await apiPatch<{ success: boolean }>("me", { name: "Sher" });
    expect(patchRes.success).toBe(true);

    const deleteRes = await apiDelete<{ success: boolean }>("me/saved/1");
    expect(deleteRes.success).toBe(true);
  });
});
