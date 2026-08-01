import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string; profileId: string }> };

export async function POST(request: Request, ctx: Ctx) {
  const { id, profileId } = await ctx.params;
  const body = await request.text();
  return adminProxy(
    request,
    `/b2b/orgs/${id}/members/${profileId}/grant`,
    { method: "POST", body },
  );
}
