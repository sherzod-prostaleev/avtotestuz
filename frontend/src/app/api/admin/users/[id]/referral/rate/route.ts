import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function PATCH(request: Request, ctx: { params: { id: string } }) {
  const body = await request.text();
  return adminProxy(request, `/users/${ctx.params.id}/referral/rate`, {
    method: "PATCH",
    body: body || "{}",
  });
}
