import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function PUT(request: Request, ctx: { params: { id: string } }) {
  const body = await request.text();
  return adminProxy(request, `/users/${ctx.params.id}/referral/balance`, {
    method: "PUT",
    body: body || "{}",
  });
}
