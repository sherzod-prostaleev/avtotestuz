import { adminProxy } from "@/lib/admin-proxy";
import { clearAdminEnrollCookie } from "@/lib/admin-auth-cookies";

export const runtime = "nodejs";

export async function POST(request: Request) {
  const body = await request.text();
  const response = await adminProxy(
    request,
    "/security/totp/confirm",
    { method: "POST", body },
    { allowEnrollmentToken: true },
  );
  // The authenticator now exists, so the backend would reject the enrollment
  // token from here on anyway — drop it rather than leave a dead credential
  // in the browser for the rest of its TTL.
  if (response.ok) clearAdminEnrollCookie(response);
  return response;
}
