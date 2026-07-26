import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const url = new URL(request.url);
  const qs = url.searchParams.toString();
  const path = qs ? `/referral/payouts?${qs}` : "/referral/payouts";
  return adminProxy(request, path, { method: "GET" });
}
