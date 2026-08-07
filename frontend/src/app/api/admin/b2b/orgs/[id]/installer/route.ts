import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

export async function GET(request: Request, ctx: Ctx) {
  const { id } = await ctx.params;
  return adminProxy(request, `/b2b/orgs/${id}/installer`, { method: "GET" });
}

export async function POST(request: Request, ctx: Ctx) {
  const { id } = await ctx.params;
  return adminProxy(request, `/b2b/orgs/${id}/installer`, { method: "POST" });
}
