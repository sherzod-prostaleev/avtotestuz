import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function POST(request: Request) {
  const body = await request.text();
  return adminProxy(request, "/content/explanations/bulk-verify", {
    method: "POST",
    body,
  });
}
