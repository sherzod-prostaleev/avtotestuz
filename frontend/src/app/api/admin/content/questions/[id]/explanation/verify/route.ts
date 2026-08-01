import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

export async function POST(request: Request, ctx: Ctx) {
  const params = await ctx.params;
  return adminProxy(request, `/content/questions/${params.id}/explanation/verify`, {
    method: "POST",
  });
}
