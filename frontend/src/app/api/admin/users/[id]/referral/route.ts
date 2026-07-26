import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function GET(request: Request, ctx: { params: { id: string } }) {
  return adminProxy(request, `/users/${ctx.params.id}/referral`, { method: "GET" });
}
