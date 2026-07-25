import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: { id: string } };

export async function GET(request: Request, { params }: Ctx) {
  return adminProxy(request, `/users/${params.id}`, { method: "GET" });
}
