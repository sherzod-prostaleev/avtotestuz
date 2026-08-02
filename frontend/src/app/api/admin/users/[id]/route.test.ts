import { afterEach, describe, expect, it, vi } from "vitest";
import { DELETE } from "./route";

afterEach(() => vi.unstubAllGlobals());

describe("DELETE /api/admin/users/:id", () => {
  it("forwards the type-to-confirm body to the admin API", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: { deleted: true } }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const id = "11111111-1111-4111-8111-111111111111";
    const body = JSON.stringify({ confirm: "+998901112233" });
    const response = await DELETE(
      new Request(`http://localhost/api/admin/users/${id}`, {
        method: "DELETE",
        headers: { Cookie: "aat=live-at", "Content-Type": "application/json" },
        body,
      }),
      { params: Promise.resolve({ id }) },
    );

    expect(response.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledWith(
      `http://localhost:8090/admin/v1/users/${id}`,
      expect.objectContaining({ method: "DELETE", body }),
    );
  });
});
