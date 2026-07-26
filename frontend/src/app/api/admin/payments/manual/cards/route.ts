import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function GET(request: Request) {
  return adminProxy(request, "/payments/manual/cards", { method: "GET" });
}

export async function POST(request: Request) {
  const body = await request.text();
  return adminProxy(request, "/payments/manual/cards", { method: "POST", body });
}
