import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ key: string }> };

export async function PATCH(request: Request, ctx: Ctx) {
  const body = await request.text();
  const { key: rawKey } = await ctx.params;
  const key = encodeURIComponent(rawKey);
  return adminProxy(request, `/settings/flags/${key}`, { method: "PATCH", body });
}
