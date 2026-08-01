import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string; sessionId: string }> };

export async function POST(request: Request, ctx: Ctx) {
  const params = await ctx.params;
  return adminProxy(
    request,
    `/users/${params.id}/sessions/${params.sessionId}/revoke`,
    { method: "POST" },
  );
}
