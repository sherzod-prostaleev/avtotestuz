import { describe, expect, it } from "vitest";
import { NextResponse } from "next/server";
import {
  AUTH_COOKIE,
  clearAuthCookies,
  cookieModeFor,
  modeForBody,
  readCookie,
  REFRESH_COOKIE,
  setAuthCookies,
  TG_MODE_COOKIE,
} from "./auth-cookies";

describe("readCookie", () => {
  it("reads and decodes the requested cookie", () => {
    const request = new Request("http://localhost", {
      headers: { Cookie: "theme=dark; at=abc%2Edef; rt=refresh-token" },
    });

    expect(readCookie(request, AUTH_COOKIE)).toBe("abc.def");
    expect(readCookie(request, REFRESH_COOKIE)).toBe("refresh-token");
  });

  it("treats malformed percent encoding as a missing cookie", () => {
    const request = new Request("http://localhost", { headers: { Cookie: "rt=%E0%A4%A" } });

    expect(() => readCookie(request, REFRESH_COOKIE)).not.toThrow();
    expect(readCookie(request, REFRESH_COOKIE)).toBeUndefined();
  });
});

const tokens = { accessToken: "a", refreshToken: "r" };

function setCookies(res: NextResponse): string[] {
  return res.headers.getSetCookie();
}

describe("telegram cookie mode", () => {
  it("site mode stays lax and never writes the marker", () => {
    const res = NextResponse.json({});
    setAuthCookies(res, tokens);
    const all = setCookies(res).join("\n").toLowerCase();
    expect(all).toContain("samesite=lax");
    expect(all).not.toContain("partitioned");
    expect(all).not.toContain(`${TG_MODE_COOKIE}=`);
  });

  it("site clear stays lax and never touches the marker", () => {
    const res = NextResponse.json({});
    clearAuthCookies(res);
    const all = setCookies(res).join("\n").toLowerCase();
    expect(all).toContain("samesite=lax");
    expect(all).not.toContain("partitioned");
    expect(all).not.toContain(`${TG_MODE_COOKIE}=`);
  });

  it("telegram mode writes None+Secure+Partitioned for at, rt and the marker", () => {
    const res = NextResponse.json({});
    setAuthCookies(res, tokens, "telegram");
    const cookies = setCookies(res);
    for (const name of ["at=", "rt=", `${TG_MODE_COOKIE}=`]) {
      const c = cookies.find((x) => x.startsWith(name) && !x.includes("Max-Age=0"))!.toLowerCase();
      expect(c).toContain("samesite=none");
      expect(c).toContain("secure");
      expect(c).toContain("partitioned");
      expect(c).toContain("httponly");
    }
  });

  it("telegram issuance first expires the unpartitioned lax at/rt, then sets the partitioned pair", () => {
    const res = NextResponse.json({});
    setAuthCookies(res, tokens, "telegram");
    const cookies = setCookies(res);
    // Expiries come first: browsers that ignore Partitioned (WebKit) treat both
    // as the same cookie, so an expiry after the set would delete the new pair.
    expect(cookies.slice(0, 2)).toEqual([
      "at=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax",
      "rt=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax",
    ]);
    expect(cookies).toHaveLength(5);
    for (const c of cookies.slice(2)) expect(c.toLowerCase()).toContain("partitioned");
    expect(res.cookies.get(AUTH_COOKIE)?.value).toBe("a");
    expect(res.cookies.get(REFRESH_COOKIE)?.value).toBe("r");
  });

  it("repeated telegram issuance on one response keeps exactly one pair of expiries", () => {
    const res = NextResponse.json({});
    setAuthCookies(res, tokens, "telegram");
    setAuthCookies(res, { accessToken: "a2", refreshToken: "r2" }, "telegram");
    const cookies = setCookies(res);
    expect(cookies).toHaveLength(5);
    expect(cookies.filter((c) => c.includes("Max-Age=0"))).toHaveLength(2);
    expect(cookies.find((c) => c.startsWith("at=a2"))).toBeDefined();
  });

  it("site issuance emits no extra expiry", () => {
    const res = NextResponse.json({});
    setAuthCookies(res, tokens);
    const cookies = setCookies(res);
    expect(cookies).toHaveLength(2);
    for (const c of cookies) expect(c.toLowerCase()).not.toContain("max-age=0");
  });

  it("telegram clear repeats Partitioned so the partitioned cookie is really removed", () => {
    const res = NextResponse.json({});
    clearAuthCookies(res, "telegram");
    const cookies = setCookies(res);
    expect(cookies).toHaveLength(3);
    for (const c of cookies) {
      expect(c.toLowerCase()).toContain("partitioned");
      expect(c.toLowerCase()).toContain("max-age=0");
    }
  });

  it("cookieModeFor reads the marker", () => {
    expect(cookieModeFor(new Request("https://x/", { headers: { cookie: "at=1" } }))).toBe("site");
    expect(cookieModeFor(new Request("https://x/"))).toBe("site");
    expect(cookieModeFor(new Request("https://x/", { headers: { cookie: `at=1; ${TG_MODE_COOKIE}=1` } }))).toBe(
      "telegram"
    );
  });

  it("modeForBody switches to telegram only for a non-empty tg_init_data string", () => {
    expect(modeForBody(JSON.stringify({ phone: "1", tg_init_data: "query_id=1" }))).toBe("telegram");
    expect(modeForBody(JSON.stringify({ phone: "1" }))).toBe("site");
    expect(modeForBody(JSON.stringify({ tg_init_data: "" }))).toBe("site");
    expect(modeForBody(JSON.stringify({ tg_init_data: 1 }))).toBe("site");
    expect(modeForBody("null")).toBe("site");
    expect(modeForBody("not json")).toBe("site");
  });
});
