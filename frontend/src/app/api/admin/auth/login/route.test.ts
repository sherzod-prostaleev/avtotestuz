import { afterEach, describe, expect, it, vi } from "vitest";
import { POST } from "./route";
import {
  ADMIN_AUTH_COOKIE,
  ADMIN_REFRESH_COOKIE,
  ADMIN_TOTP_ENROLL_COOKIE,
} from "@/lib/admin-auth-cookies";

afterEach(() => {
  vi.unstubAllGlobals();
});

function loginRequest(): Request {
  return new Request("http://localhost/api/admin/auth/login", {
    method: "POST",
    body: JSON.stringify({ email: "root@drivergo.uz", password: "hunter2hunter2" }),
  });
}

const setupRequiredBody = {
  data: {
    totp_setup_required: true,
    enrollment_token: "enroll.jwt.value",
    expires_in: 900,
    enroll_url: "/admin/v1/security/totp/enroll",
  },
  error: {
    code: "totp_setup_required",
    message: "two-factor authentication must be enrolled before signing in",
  },
};

describe("POST /api/admin/auth/login", () => {
  it("stores the enrollment token in an httpOnly cookie and strips it from the body", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify(setupRequiredBody), { status: 403 }),
      ),
    );

    const response = await POST(loginRequest());
    const json = await response.json();

    expect(response.status).toBe(403);
    expect(json.error.code).toBe("totp_setup_required");
    expect(json.data.totp_setup_required).toBe(true);
    expect(json.data.enrollment_token).toBeUndefined();
    expect(JSON.stringify(json)).not.toContain("enroll.jwt.value");

    const cookie = response.cookies.get(ADMIN_TOTP_ENROLL_COOKIE);
    expect(cookie?.value).toBe("enroll.jwt.value");
    expect(cookie?.httpOnly).toBe(true);
    expect(cookie?.maxAge).toBe(900);
    // Scoped so the browser only ever sends it to the two enrollment routes.
    expect(cookie?.path).toBe("/api/admin/security/totp");
    // No session was granted, so nothing may look like one.
    expect(response.cookies.get(ADMIN_AUTH_COOKIE)?.value).toBe("");
    expect(response.cookies.get(ADMIN_REFRESH_COOKIE)?.value).toBe("");
  });

  it("caps the enrollment cookie lifetime at the token TTL", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            ...setupRequiredBody,
            data: { ...setupRequiredBody.data, expires_in: 86400 },
          }),
          { status: 403 },
        ),
      ),
    );

    const response = await POST(loginRequest());

    expect(response.cookies.get(ADMIN_TOTP_ENROLL_COOKIE)?.maxAge).toBe(900);
  });

  it("sets admin session cookies and drops any enrollment cookie on success", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({ data: { tokens: { access_token: "aat.1", refresh_token: "art.1" } } }),
          { status: 200 },
        ),
      ),
    );

    const response = await POST(loginRequest());

    expect(response.cookies.get(ADMIN_AUTH_COOKIE)?.value).toBe("aat.1");
    expect(response.cookies.get(ADMIN_REFRESH_COOKIE)?.value).toBe("art.1");
    expect(response.cookies.get(ADMIN_TOTP_ENROLL_COOKIE)?.value).toBe("");
    expect(response.cookies.get(ADMIN_TOTP_ENROLL_COOKIE)?.maxAge).toBe(0);
  });

  it("passes invalid_credentials through without minting any credential cookie", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({ error: { code: "invalid_credentials", message: "invalid" } }),
          { status: 401 },
        ),
      ),
    );

    const response = await POST(loginRequest());
    const json = await response.json();

    expect(response.status).toBe(401);
    expect(json.error.code).toBe("invalid_credentials");
    expect(response.cookies.get(ADMIN_AUTH_COOKIE)?.value).toBe("");
    expect(response.cookies.get(ADMIN_TOTP_ENROLL_COOKIE)?.value).toBe("");
  });

  it("does not mint an enrollment cookie when the backend omits the token", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            data: { totp_setup_required: true },
            error: { code: "totp_setup_required", message: "enroll first" },
          }),
          { status: 403 },
        ),
      ),
    );

    const response = await POST(loginRequest());

    expect(response.status).toBe(403);
    expect(response.cookies.get(ADMIN_TOTP_ENROLL_COOKIE)?.maxAge).toBe(0);
  });
});
