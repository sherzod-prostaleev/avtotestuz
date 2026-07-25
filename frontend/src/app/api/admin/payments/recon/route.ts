import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const url = new URL(request.url);
  const hours = url.searchParams.get("hours") ?? "24";
  return adminProxy(request, `/payments/recon?hours=${encodeURIComponent(hours)}`, { method: "GET" });
}
