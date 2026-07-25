import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const url = new URL(request.url);
  const qs = url.searchParams.toString();
  const path = qs ? `/content/explanations?${qs}` : "/content/explanations";
  return adminProxy(request, path, { method: "GET" });
}
