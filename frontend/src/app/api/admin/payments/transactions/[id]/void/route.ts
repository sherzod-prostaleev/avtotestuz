import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

export async function POST(request: Request, ctx: Ctx) {
  const params = await ctx.params;
  const body = await request.text();
  return adminProxy(request, `/payments/transactions/${params.id}/void`, {
    method: "POST",
    body,
  });
}
