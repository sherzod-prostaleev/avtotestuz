import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: { provider: string } };

export async function PATCH(request: Request, { params }: Ctx) {
  const body = await request.text();
  return adminProxy(request, `/payments/providers/${params.provider}`, {
    method: "PATCH",
    body,
  });
}
