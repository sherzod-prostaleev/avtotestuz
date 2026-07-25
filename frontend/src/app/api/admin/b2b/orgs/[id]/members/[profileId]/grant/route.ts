import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: { id: string; profileId: string } };

export async function POST(request: Request, ctx: Ctx) {
  const body = await request.text();
  return adminProxy(
    request,
    `/b2b/orgs/${ctx.params.id}/members/${ctx.params.profileId}/grant`,
    { method: "POST", body },
  );
}
