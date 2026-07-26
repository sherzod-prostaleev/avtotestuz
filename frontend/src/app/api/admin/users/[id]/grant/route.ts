import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: { id: string } };

export async function POST(request: Request, ctx: Ctx) {
  const body = await request.text();
  return adminProxy(request, `/users/${ctx.params.id}/grant`, {
    method: "POST",
    body,
  });
}
