import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function GET(request: Request) {
  return adminProxy(request, "/support/banner", { method: "GET" });
}

export async function PUT(request: Request) {
  const body = await request.text();
  return adminProxy(request, "/support/banner", { method: "PUT", body });
}
