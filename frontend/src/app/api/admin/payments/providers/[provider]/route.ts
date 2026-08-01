import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ provider: string }> };

export async function PATCH(request: Request, ctx: Ctx) {
  const params = await ctx.params;
  const body = await request.text();
  return adminProxy(request, `/payments/providers/${params.provider}`, {
    method: "PATCH",
    body,
  });
}
