import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const url = new URL(request.url);
  const qs = url.searchParams.toString();
  const path = qs ? `/support/tickets?${qs}` : "/support/tickets";
  return adminProxy(request, path, { method: "GET" });
}
