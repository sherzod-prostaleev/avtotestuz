import { adminProxy } from "@/lib/admin-proxy";

export const runtime = "nodejs";

export async function POST(request: Request) {
  // One of the two routes an admin locked out by ADMIN_TOTP_ENFORCE can reach
  // with the scoped enrollment cookie instead of a session.
  return adminProxy(
    request,
    "/security/totp/enroll",
    { method: "POST", body: "{}" },
    { allowEnrollmentToken: true },
  );
}
