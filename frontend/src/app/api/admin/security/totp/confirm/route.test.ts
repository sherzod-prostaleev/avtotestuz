import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { POST } from "./route";
import {
  ADMIN_AUTH_COOKIE,
  ADMIN_REFRESH_COOKIE,
  ADMIN_TOTP_ENROLL_COOKIE,
} from "@/lib/admin-auth-cookies";
import { resetRefreshLockForTests } from "@/lib/refresh-lock";

beforeEach(() => {
  resetRefreshLockForTests();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function confirmRequest(cookie: string): Request {
  return new Request("http://localhost/api/admin/security/totp/confirm", {
    method: "POST",
    headers: { Cookie: cookie, "Content-Type": "application/json" },
    body: JSON.stringify({ code: "123456" }),
  });
}

describe("POST /api/admin/security/totp/confirm", () => {
  it("confirms on the enrollment cookie and burns it afterwards", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ data: { totp_enabled: true } }), { status: 200 }),
      );
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(confirmRequest("ate=enroll-token"));

    expect(response.status).toBe(200);
    expect(
      (fetchMock.mock.calls[0][1].headers as Record<string, string>).Authorization,
    ).toBe("Bearer enroll-token");
    expect(response.cookies.get(ADMIN_TOTP_ENROLL_COOKIE)?.maxAge).toBe(0);
  });

  it("still works on a normal admin session and leaves it alone", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ data: { totp_enabled: true } }), { status: 200 }),
      );
    vi.stubGlobal("fetch", fetchMock);

    const response = await POST(confirmRequest("aat=live-at; art=live-rt"));

    expect(response.status).toBe(200);
    expect(
      (fetchMock.mock.calls[0][1].headers as Record<string, string>).Authorization,
    ).toBe("Bearer live-at");
    expect(response.cookies.get(ADMIN_AUTH_COOKIE)).toBeUndefined();
    expect(response.cookies.get(ADMIN_REFRESH_COOKIE)).toBeUndefined();
  });

  it("does not burn the enrollment cookie when the code is wrong", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ error: { code: "invalid_totp" } }), { status: 400 }),
        ),
    );

    const response = await POST(confirmRequest("ate=enroll-token"));

    expect(response.status).toBe(400);
    expect(response.cookies.get(ADMIN_TOTP_ENROLL_COOKIE)).toBeUndefined();
  });
});
