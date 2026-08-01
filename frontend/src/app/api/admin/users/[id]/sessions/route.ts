import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

export async function GET(request: Request, ctx: Ctx) {
  const params = await ctx.params;
  return adminProxy(request, `/users/${params.id}/sessions`, { method: "GET" });
}

export async function POST(request: Request, ctx: Ctx) {
  const params = await ctx.params;
  return adminProxy(request, `/users/${params.id}/sessions/revoke-all`, {
    method: "POST",
  });
}
