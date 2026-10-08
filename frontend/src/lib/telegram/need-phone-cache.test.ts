import { afterEach, describe, expect, it, vi } from "vitest";
import { forgetNeedPhone, recallNeedPhone, rememberNeedPhone } from "./need-phone-cache";

afterEach(() => {
  sessionStorage.clear();
  vi.restoreAllMocks();
});

describe("need_phone cache", () => {
  it("remembers the verdict for the launching Telegram user only", () => {
    rememberNeedPhone(7, "Ali");
    expect(recallNeedPhone(7)).toEqual({ firstName: "Ali" });
    expect(recallNeedPhone(8)).toBeNull();
  });

  it("keeps an empty first name as a hit, not a miss", () => {
    rememberNeedPhone(7, "");
    expect(recallNeedPhone(7)).toEqual({ firstName: "" });
  });

  it("is a miss without a user id", () => {
    rememberNeedPhone(undefined, "Ali");
    expect(recallNeedPhone(undefined)).toBeNull();
  });

  it("forget drops every remembered user", () => {
    rememberNeedPhone(7, "Ali");
    rememberNeedPhone(8, "Vali");
    sessionStorage.setItem("unrelated", "1");
    forgetNeedPhone();
    expect(recallNeedPhone(7)).toBeNull();
    expect(recallNeedPhone(8)).toBeNull();
    expect(sessionStorage.getItem("unrelated")).toBe("1");
  });

  it("treats a corrupt entry as a miss", () => {
    sessionStorage.setItem("tg-need-phone:7", "{not json");
    expect(recallNeedPhone(7)).toBeNull();
  });

  it("never throws when storage is blocked", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    expect(() => rememberNeedPhone(7, "Ali")).not.toThrow();
    expect(recallNeedPhone(7)).toBeNull();
    expect(() => forgetNeedPhone()).not.toThrow();
  });
});
