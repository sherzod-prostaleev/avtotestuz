import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function PUT(request: Request, ctx: { params: Promise<{ id: string }> }) {
  const { id } = await ctx.params;
  const body = await request.text();
  return adminProxy(request, `/users/${id}/referral/balance`, {
    method: "PUT",
    body: body || "{}",
  });
}
