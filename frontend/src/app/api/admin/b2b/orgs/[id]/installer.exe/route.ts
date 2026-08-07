import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

// The backend streams a binary (base station .exe + appended per-school
// config trailer). adminProxy's forwardResponse already special-cases
// application/octet-stream — it reads the upstream body as an ArrayBuffer
// instead of JSON-parsing it, and forwards Content-Disposition unchanged —
// so this route just needs to pass the request (and its ?locale= query
// string) through untouched.
export async function GET(request: Request, ctx: Ctx) {
  const { id } = await ctx.params;
  const url = new URL(request.url);
  const qs = url.searchParams.toString();
  const path = qs ? `/b2b/orgs/${id}/installer.exe?${qs}` : `/b2b/orgs/${id}/installer.exe`;
  return adminProxy(request, path, { method: "GET" });
}
