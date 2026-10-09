import { afterEach, describe, expect, it } from "vitest";
import { isMaskedPhone, rememberSignedInAs, takeSignedInAs } from "./signed-in-notice";
import { rememberResetPhone, takeResetPhone } from "./reset-phone-handoff";

afterEach(() => window.sessionStorage.clear());

describe("signed-in notice hand-off", () => {
  it("keeps only the masked form and gives it out once", () => {
    rememberSignedInAs("+998 90 ••• •• 67");
    expect(takeSignedInAs()).toBe("+998 90 ••• •• 67");
    expect(takeSignedInAs()).toBeNull();
  });

  it("drops anything that is not a masked +998 number", () => {
    for (const bad of ["+998901234567", "<b>x</b>", "", null, undefined, 42, "+998 90 123 45 67"]) {
      rememberSignedInAs(bad);
      expect(takeSignedInAs()).toBeNull();
      expect(isMaskedPhone(bad)).toBe(false);
    }
    window.sessionStorage.setItem("drivergo:signedInAs", "+998901234567");
    expect(takeSignedInAs()).toBeNull();
  });
});

describe("reset phone hand-off", () => {
  it("carries a national number once and nothing else", () => {
    rememberResetPhone("90 111 22 33");
    expect(takeResetPhone()).toBe("901112233");
    expect(takeResetPhone()).toBeNull();
    rememberResetPhone("12");
    expect(takeResetPhone()).toBeNull();
    window.sessionStorage.setItem("drivergo:resetPhone", "<script>");
    expect(takeResetPhone()).toBeNull();
  });
});
