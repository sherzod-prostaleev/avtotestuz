import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: { id: string } };

export async function GET(request: Request, ctx: Ctx) {
  return adminProxy(request, `/b2b/orgs/${ctx.params.id}/stats`, { method: "GET" });
}
