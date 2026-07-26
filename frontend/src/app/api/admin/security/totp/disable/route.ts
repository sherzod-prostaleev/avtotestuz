import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function POST(request: Request) {
  const body = await request.text();
  return adminProxy(request, "/security/totp/disable", { method: "POST", body });
}
