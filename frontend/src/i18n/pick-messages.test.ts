import { describe, expect, it } from "vitest";
import uzLatn from "../../messages/uz-Latn.json";
import { pickMessages } from "./pick-messages";
import {
  ADMIN_NAMESPACES,
  APP_NAMESPACES,
  AUTH_NAMESPACES,
  COMMON_NAMESPACES,
  KIOSK_NAMESPACES,
  PUBLIC_NAMESPACES,
  SESSION_NAMESPACES,
} from "./namespaces";

describe("pickMessages", () => {
  it("keeps only requested namespaces", () => {
    const picked = pickMessages(uzLatn, ["Landing", "Errors"]);
    expect(Object.keys(picked).sort()).toEqual(["Errors", "Landing"]);
    expect(picked.Landing).toEqual(uzLatn.Landing);
  });

  it("does not put admin namespaces on the public client payload", () => {
    expect(PUBLIC_NAMESPACES.some((name) => name.startsWith("Admin"))).toBe(false);
    expect(PUBLIC_NAMESPACES).not.toContain("Sidebar");
  });

  it("keeps Session strings on the app shell for exam-mockup QuestionCard", () => {
    expect(APP_NAMESPACES).toContain("Session");
  });

  it("covers chrome namespaces on every route group", () => {
    for (const group of [PUBLIC_NAMESPACES, AUTH_NAMESPACES, APP_NAMESPACES, SESSION_NAMESPACES, ADMIN_NAMESPACES, KIOSK_NAMESPACES]) {
      for (const name of COMMON_NAMESPACES) {
        expect(group).toContain(name);
      }
    }
  });
});
