import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

export async function GET(request: Request, ctx: Ctx) {
  const params = await ctx.params;
  return adminProxy(request, `/support/tickets/${params.id}`, { method: "GET" });
}

export async function PATCH(request: Request, ctx: Ctx) {
  const params = await ctx.params;
  const body = await request.text();
  return adminProxy(request, `/support/tickets/${params.id}`, { method: "PATCH", body });
}
