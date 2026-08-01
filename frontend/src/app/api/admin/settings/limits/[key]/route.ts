import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function PATCH(
  request: Request,
  ctx: { params: Promise<{ key: string }> },
) {
  const { key } = await ctx.params;
  const body = await request.text();
  return adminProxy(request, `/settings/limits/${key}`, { method: "PATCH", body });
}
