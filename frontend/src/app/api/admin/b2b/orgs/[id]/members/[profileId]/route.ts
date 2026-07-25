import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: { id: string; profileId: string } };

export async function DELETE(request: Request, ctx: Ctx) {
  return adminProxy(request, `/b2b/orgs/${ctx.params.id}/members/${ctx.params.profileId}`, {
    method: "DELETE",
  });
}

export async function PATCH(request: Request, ctx: Ctx) {
  const body = await request.text();
  return adminProxy(request, `/b2b/orgs/${ctx.params.id}/members/${ctx.params.profileId}`, {
    method: "PATCH",
    body,
  });
}
