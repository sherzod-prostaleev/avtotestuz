import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function PATCH(request: Request, ctx: { params: Promise<{ id: string }> }) {
  const { id } = await ctx.params;
  const body = await request.text();
  return adminProxy(request, `/payments/manual/cards/${id}`, { method: "PATCH", body });
}

export async function DELETE(request: Request, ctx: { params: Promise<{ id: string }> }) {
  const { id } = await ctx.params;
  return adminProxy(request, `/payments/manual/cards/${id}`, { method: "DELETE" });
}
