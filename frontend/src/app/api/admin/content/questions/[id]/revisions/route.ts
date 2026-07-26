import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: { id: string } };

export async function GET(request: Request, ctx: Ctx) {
  return adminProxy(request, `/content/questions/${ctx.params.id}/revisions`, {
    method: "GET",
  });
}
