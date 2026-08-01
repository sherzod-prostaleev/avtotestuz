import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function POST(request: Request, ctx: { params: Promise<{ id: string }> }) {
  const { id } = await ctx.params;
  const body = await request.text();
  return adminProxy(request, `/referral/payouts/${id}/paid`, {
    method: "POST",
    body: body || "{}",
  });
}
