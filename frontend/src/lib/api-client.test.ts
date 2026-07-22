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
