import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function POST(request: Request) {
  return adminProxy(request, "/security/totp/enroll", { method: "POST", body: "{}" });
}
