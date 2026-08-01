import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string; profileId: string }> };

export async function DELETE(request: Request, ctx: Ctx) {
  const { id, profileId } = await ctx.params;
  return adminProxy(request, `/b2b/orgs/${id}/members/${profileId}`, {
    method: "DELETE",
  });
}

export async function PATCH(request: Request, ctx: Ctx) {
  const { id, profileId } = await ctx.params;
  const body = await request.text();
  return adminProxy(request, `/b2b/orgs/${id}/members/${profileId}`, {
    method: "PATCH",
    body,
  });
}
