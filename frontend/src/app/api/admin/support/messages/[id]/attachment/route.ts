import { adminProxy } from "@/lib/admin-proxy";

type Ctx = { params: Promise<{ id: string }> };

/** Authenticated binary download for a support message attachment. */
export async function GET(request: Request, ctx: Ctx) {
  const { id } = await ctx.params;
  return adminProxy(request, `/support/messages/${id}/attachment`, { method: "GET" });
}
