import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export function GET(request: Request) {
  return adminProxy(request, "/me", { method: "GET" });
}
