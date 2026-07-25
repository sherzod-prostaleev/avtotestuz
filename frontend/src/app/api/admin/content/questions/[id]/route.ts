import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: { id: string } };

export async function GET(request: Request, { params }: Ctx) {
  return adminProxy(request, `/content/questions/${params.id}`, { method: "GET" });
}

export async function PATCH(request: Request, { params }: Ctx) {
  const body = await request.text();
  return adminProxy(request, `/content/questions/${params.id}`, {
    method: "PATCH",
    body,
  });
}
