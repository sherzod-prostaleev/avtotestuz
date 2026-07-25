import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: { id: string } };

export async function GET(request: Request, { params }: Ctx) {
  return adminProxy(request, `/users/${params.id}/sessions`, { method: "GET" });
}

export async function POST(request: Request, { params }: Ctx) {
  return adminProxy(request, `/users/${params.id}/sessions/revoke-all`, {
    method: "POST",
  });
}
