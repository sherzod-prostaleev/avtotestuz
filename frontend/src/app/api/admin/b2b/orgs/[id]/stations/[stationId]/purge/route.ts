import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string; stationId: string }> };

// The irreversible counterpart to DELETE ../[stationId], which only revokes.
// Purge removes the station row and the shadow profile it practised under,
// taking that PC's sessions, saved questions and progress with it.
export async function DELETE(request: Request, ctx: Ctx) {
  const { id, stationId } = await ctx.params;
  return adminProxy(request, `/b2b/orgs/${id}/stations/${stationId}/purge`, { method: "DELETE" });
}
