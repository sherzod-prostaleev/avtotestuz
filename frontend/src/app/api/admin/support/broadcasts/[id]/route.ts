import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

export async function GET(request: Request, ctx: Ctx) {
  const { id } = await ctx.params;
  return adminProxy(request, `/support/broadcasts/${encodeURIComponent(id)}`, { method: "GET" });
}
