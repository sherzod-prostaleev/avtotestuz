import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: Promise<{ id: string }> };

// Reports from machines that never became stations. Separate from
// /stations because those rows have no station id -- a PC blocked at
// enrolment is exactly the one that cannot hold a station token, and its
// report is filed against the school instead.
export async function GET(request: Request, ctx: Ctx) {
  const { id } = await ctx.params;
  return adminProxy(request, `/b2b/orgs/${id}/enroll-failures`);
}
