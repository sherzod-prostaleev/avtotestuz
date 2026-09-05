import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";
type Ctx = { params: Promise<{ id: string }> };
export async function GET(request: Request, ctx: Ctx) {
  const { id } = await ctx.params;
  return adminProxy(request, `/b2b/orgs/${id}/mobile-promo`, { method: "GET" });
}
export async function PUT(request: Request, ctx: Ctx) {
  const { id } = await ctx.params;
  return adminProxy(request, `/b2b/orgs/${id}/mobile-promo`, { method: "PUT", body: await request.text() });
}
