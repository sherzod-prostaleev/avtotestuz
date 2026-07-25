import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: { key: string } };

export async function PATCH(request: Request, ctx: Ctx) {
  const body = await request.text();
  const key = encodeURIComponent(ctx.params.key);
  return adminProxy(request, `/settings/flags/${key}`, { method: "PATCH", body });
}
