import { afterEach, describe, expect, it, vi } from "vitest";
import { DELETE } from "./route";

afterEach(() => vi.unstubAllGlobals());

describe("DELETE /api/admin/b2b/orgs/:id", () => {
  it("forwards the exact organization-name confirmation", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: { deleted: true } }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const id = "22222222-2222-4222-8222-222222222222";
    const body = JSON.stringify({ confirm: "Chilonzor Avtomaktab" });
    const response = await DELETE(
      new Request(`http://localhost/api/admin/b2b/orgs/${id}`, {
        method: "DELETE",
        headers: { Cookie: "aat=live-at", "Content-Type": "application/json" },
        body,
      }),
      { params: Promise.resolve({ id }) },
    );

    expect(response.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledWith(
      `http://localhost:8090/admin/v1/b2b/orgs/${id}`,
      expect.objectContaining({ method: "DELETE", body }),
    );
  });
});
