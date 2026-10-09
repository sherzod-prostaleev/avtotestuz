import { NextResponse } from "next/server";
import { backendFetch } from "@/lib/backend";
import { extractTokenPair, readBackendJson } from "@/lib/backend-response";
import { setAuthCookies } from "@/lib/auth-cookies";
import { buildClientIPAssertionHeaders } from "@/lib/client-ip-assertion";
import { rejectCrossSite } from "@/lib/same-origin";

export const runtime = "nodejs";

function unavailableResponse() {
  return NextResponse.json(
    { error: { code: "network_error", message: "service temporarily unavailable" } },
    { status: 502 }
  );
}

/**
 * The Mini App's one-tap «📱 Raqam bilan davom etish»: launch data plus
 * Telegram's signed phone share sign in to — or create — the learner with
 * that phone. Mini App only, so the cookies are Telegram-mode.
 */
export async function POST(request: Request) {
  const refused = rejectCrossSite(request);
  if (refused) return refused;

  let backendRes: Response;
  let data: unknown;
  try {
    backendRes = await backendFetch("/auth/telegram/webapp/phone", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...buildClientIPAssertionHeaders(request, "/auth/telegram/webapp/phone"),
      },
      body: await request.text(),
    });
    data = await readBackendJson(backendRes);
  } catch {
    return unavailableResponse();
  }
  if (!backendRes.ok) {
    return NextResponse.json(data, { status: backendRes.status });
  }
  let tokens: { accessToken: string; refreshToken: string };
  try {
    tokens = extractTokenPair(data);
  } catch {
    return unavailableResponse();
  }
  const payload = (data as { data?: { must_change_password?: unknown; created?: unknown } }).data;
  const response = NextResponse.json(
    { data: { ok: true, must_change_password: payload?.must_change_password === true, created: payload?.created === true } },
    { status: 200 }
  );
  setAuthCookies(response, tokens, "telegram");
  return response;
}
