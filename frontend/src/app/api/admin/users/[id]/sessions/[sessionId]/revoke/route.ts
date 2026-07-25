import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: { id: string; sessionId: string } };

export async function POST(request: Request, { params }: Ctx) {
  return adminProxy(
    request,
    `/users/${params.id}/sessions/${params.sessionId}/revoke`,
    { method: "POST" },
  );
}
