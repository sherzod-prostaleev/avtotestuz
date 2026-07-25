import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

type Ctx = { params: { id: string } };

export async function POST(request: Request, { params }: Ctx) {
  return adminProxy(request, `/content/questions/${params.id}/explanation/verify`, {
    method: "POST",
  });
}
