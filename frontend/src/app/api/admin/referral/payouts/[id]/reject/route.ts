import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function POST(request: Request, ctx: { params: { id: string } }) {
  const body = await request.text();
  return adminProxy(request, `/referral/payouts/${ctx.params.id}/reject`, {
    method: "POST",
    body: body || "{}",
  });
}
