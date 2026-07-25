import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function PATCH(
  request: Request,
  ctx: { params: { key: string } },
) {
  const body = await request.text();
  return adminProxy(request, `/settings/limits/${ctx.params.key}`, { method: "PATCH", body });
}
