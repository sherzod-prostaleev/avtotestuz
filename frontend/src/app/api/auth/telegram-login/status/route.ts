import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { readBackendJson } from "@/lib/backend-response";
import { buildClientIPAssertionHeaders } from "@/lib/client-ip-assertion";
import { TELEGRAM_LOGIN_TOKEN_HEADER, isLoginToken, readTelegramLoginSecret } from "@/lib/telegram-login-cookie";

export const runtime = "nodejs";

/**
 * The waiting page's poll: pending | approved | cancelled | blocked | invalid.
 * Cookie-bound — without this browser's secret every answer is "invalid" —
 * and it never carries the phone or the name.
 */
export async function GET(request: Request) {
  // A header, not ?token=: request URLs end up in access logs, and the token
  // is stored nowhere else in clear.
  const token = request.headers.get(TELEGRAM_LOGIN_TOKEN_HEADER) ?? "";
  const secret = readTelegramLoginSecret(request);
  const noStore = { "Cache-Control": "no-store" };
  if (!isLoginToken(token) || !secret) {
    return NextResponse.json({ data: { state: "invalid" } }, { status: 200, headers: noStore });
  }
  try {
    const backendRes = await backendFetch("/auth/telegram-login/status", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...buildClientIPAssertionHeaders(request, "/auth/telegram-login/status"),
      },
      body: JSON.stringify({ token, browser_secret: secret }),
    });
    const data = await readBackendJson(backendRes);
    return NextResponse.json(data, { status: backendRes.status, headers: noStore });
  } catch {
    return NextResponse.json(
      { error: { code: "network_error", message: "service temporarily unavailable" } },
      { status: 502, headers: noStore }
    );
  }
}
