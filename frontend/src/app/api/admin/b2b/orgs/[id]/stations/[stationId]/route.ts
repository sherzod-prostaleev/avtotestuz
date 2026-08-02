import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string; stationId: string }> };

export async function DELETE(request: Request, ctx: Ctx) {
  const { id, stationId } = await ctx.params;
  return adminProxy(request, `/b2b/orgs/${id}/stations/${stationId}`, { method: "DELETE" });
}
