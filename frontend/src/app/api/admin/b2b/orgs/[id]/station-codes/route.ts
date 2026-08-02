import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

export async function POST(request: Request, ctx: Ctx) {
  const { id } = await ctx.params;
  const body = await request.text();
  return adminProxy(request, `/b2b/orgs/${id}/station-codes`, { method: "POST", body });
}
