import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

export async function GET(request: Request, ctx: Ctx) {
  const params = await ctx.params;
  return adminProxy(request, `/users/${params.id}`, { method: "GET" });
}

export async function DELETE(request: Request, ctx: Ctx) {
  const params = await ctx.params;
  return adminProxy(request, `/users/${params.id}`, {
    method: "DELETE",
    body: await request.text(),
  });
}
