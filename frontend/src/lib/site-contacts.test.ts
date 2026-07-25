import { describe, expect, it } from "vitest";
import { contactOrFallback } from "./site-contacts";

describe("contactOrFallback", () => {
  it("prefers non-empty API values", () => {
    expect(contactOrFallback(" +998 90 ", "+998 71")).toBe("+998 90");
  });

  it("falls back when API empty", () => {
    expect(contactOrFallback("", "+998 71")).toBe("+998 71");
    expect(contactOrFallback("   ", "+998 71")).toBe("+998 71");
    expect(contactOrFallback(undefined, "+998 71")).toBe("+998 71");
  });
});
