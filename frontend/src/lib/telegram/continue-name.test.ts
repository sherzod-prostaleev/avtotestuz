import { describe, expect, it } from "vitest";
import { continueName } from "./continue-name";

describe("continueName", () => {
  it("uses the first word of the Telegram first name", () => {
    expect(continueName("Ali Valiyev")).toBe("Ali");
  });
  it("caps a long single word with an ellipsis", () => {
    expect(continueName("Abdurahmonbekjonovich")).toBe("Abdurahmonbekjon…");
  });
  it("is null for an empty or blank name", () => {
    expect(continueName("")).toBeNull();
    expect(continueName("   ")).toBeNull();
  });
  it("does not split a surrogate pair at the cap", () => {
    const name = "Abdurahmonbekjo😀x";
    expect(continueName(name)).toBe("Abdurahmonbekjo😀…");
  });
});
