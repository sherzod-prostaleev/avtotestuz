import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

export async function POST(request: Request, context: Ctx) {
  const { id } = await context.params;
  const body = await request.text();
  return adminProxy(request, `/support/conversations/${id}/messages`, { method: "POST", body });
}
