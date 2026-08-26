import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string; stationId: string }> };

// The way back from DELETE ../[stationId], which only revokes. The PC keeps its
// station id and its sealed key across a revoke, so putting the row back is all
// it takes -- the agent recovers on its own at its next token renewal, within
// two minutes, with nobody touching the machine.
//
// POST rather than DELETE because this one puts something back, and it can fail
// on a full licence: the seat cap is enforced here too, or revoking ten PCs and
// reactivating all ten would be a hole straight through it.
export async function POST(request: Request, ctx: Ctx) {
  const { id, stationId } = await ctx.params;
  return adminProxy(request, `/b2b/orgs/${id}/stations/${stationId}/reactivate`, { method: "POST" });
}
