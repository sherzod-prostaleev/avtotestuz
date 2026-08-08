import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

export async function GET(request: Request, context: Ctx) {
  const { id } = await context.params;
  const url = new URL(request.url);
  const qs = url.searchParams.toString();
  const path = qs ? `/support/conversations/${id}?${qs}` : `/support/conversations/${id}`;
  return adminProxy(request, path, { method: "GET" });
}

export async function PATCH(request: Request, context: Ctx) {
  const { id } = await context.params;
  const body = await request.text();
  return adminProxy(request, `/support/conversations/${id}`, { method: "PATCH", body });
}
