import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function GET(request: Request) {
  return adminProxy(request, "/cms/home", { method: "GET" });
}

export async function PUT(request: Request) {
  const body = await request.text();
  return adminProxy(request, "/cms/home", { method: "PUT", body });
}
