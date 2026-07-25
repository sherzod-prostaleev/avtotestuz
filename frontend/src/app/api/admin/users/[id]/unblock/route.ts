import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: { id: string } };

export async function POST(request: Request, { params }: Ctx) {
  const body = await request.text();
  return adminProxy(request, `/users/${params.id}/unblock`, {
    method: "POST",
    body,
  });
}
