import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const url = new URL(request.url);
  const qs = url.searchParams.toString();
  const path = qs ? `/security/audit?${qs}` : "/security/audit";
  return adminProxy(request, path, { method: "GET" });
}
