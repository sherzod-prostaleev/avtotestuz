import { describe, expect, it } from "vitest";
import { AUTH_COOKIE, readCookie, REFRESH_COOKIE } from "./auth-cookies";

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
