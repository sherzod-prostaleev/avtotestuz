import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

export async function GET(request: Request, ctx: Ctx) {
  const params = await ctx.params;
  return adminProxy(request, `/payments/transactions/${params.id}`, { method: "GET" });
}
