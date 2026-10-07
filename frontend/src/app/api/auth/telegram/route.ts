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

// Exchanges Telegram Mini App launch data for a session. A linked Telegram
// account gets Telegram-mode cookies; an unlinked one gets need_phone and goes
// through the ordinary phone sign-in.
export async function POST(request: Request) {
  const refused = rejectCrossSite(request);
  if (refused) return refused;

  let backendRes: Response;
  let data: unknown;
  try {
    backendRes = await backendFetch("/auth/telegram/webapp", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...buildClientIPAssertionHeaders(request, "/auth/telegram/webapp"),
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

  const payload = (
    data as { data?: { need_phone?: unknown; first_name?: unknown; must_change_password?: unknown } }
  ).data;
  if (payload?.need_phone === true) {
    return NextResponse.json(
      { data: { need_phone: true, first_name: typeof payload.first_name === "string" ? payload.first_name : "" } },
      { status: 200 }
    );
  }

  let tokens: { accessToken: string; refreshToken: string };
  try {
    tokens = extractTokenPair(data);
  } catch {
    return unavailableResponse();
  }
  const response = NextResponse.json(
    { data: { ok: true, must_change_password: payload?.must_change_password === true } },
    { status: 200 }
  );
  setAuthCookies(response, tokens, "telegram");
  return response;
}
