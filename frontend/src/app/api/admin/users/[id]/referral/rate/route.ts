import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function PATCH(request: Request, ctx: { params: Promise<{ id: string }> }) {
  const { id } = await ctx.params;
  const body = await request.text();
  return adminProxy(request, `/users/${id}/referral/rate`, {
    method: "PATCH",
    body: body || "{}",
  });
}
