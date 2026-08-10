import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const url = new URL(request.url);
  const qs = url.searchParams.toString();
  return adminProxy(request, `/support/broadcasts${qs ? `?${qs}` : ""}`, { method: "GET" });
}

export async function POST(request: Request) {
  const body = await request.text();
  const headers: Record<string, string> = {};
  const idem = request.headers.get("Idempotency-Key");
  if (idem) headers["Idempotency-Key"] = idem;
  return adminProxy(request, "/support/broadcasts", { method: "POST", body, headers });
}
