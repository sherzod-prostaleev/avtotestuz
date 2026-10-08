import { describe, expect, it } from "vitest";
import {
  SUPPORT_TELEGRAM_FALLBACK,
  compactPhoneTel,
  contactOrFallback,
  resolvePhonePair,
  supportTelegramUrl,
} from "./site-contacts";

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

describe("compactPhoneTel", () => {
  it("keeps digits and a leading plus", () => {
    expect(compactPhoneTel("+998 50 500 10 45")).toBe("+998505001045");
    expect(compactPhoneTel("90-123-45-67")).toBe("901234567");
  });
});

describe("resolvePhonePair", () => {
  it("uses both CMS fields when present", () => {
    expect(resolvePhonePair("+998 50", "+99850", "fb", "fbTel")).toEqual({
      phone: "+998 50",
      phoneTel: "+99850",
    });
  });

  it("cross-fills display from tel when phone empty", () => {
    expect(resolvePhonePair("", "+998505001045", "+998 71 200 00 00", "+998712000000")).toEqual({
      phone: "+998505001045",
      phoneTel: "+998505001045",
    });
  });

  it("cross-fills tel from display when phoneTel empty", () => {
    expect(resolvePhonePair("+998 50 500 10 45", "", "fb", "fbTel")).toEqual({
      phone: "+998 50 500 10 45",
      phoneTel: "+998505001045",
    });
  });

  it("falls back only when both CMS fields are empty", () => {
    expect(resolvePhonePair("", "", "fb", "fbTel")).toEqual({
      phone: "fb",
      phoneTel: "fbTel",
    });
  });
});

describe("supportTelegramUrl", () => {
  it("uses an https t.me link from the CMS", () => {
    expect(supportTelegramUrl(" https://t.me/DriverGoSupport ")).toBe("https://t.me/DriverGoSupport");
  });
  it("falls back for anything else", () => {
    for (const value of ["", undefined, null, "http://t.me/x", "https://evil.com/x", "javascript:alert(1)", "https://t.me/"]) {
      expect(supportTelegramUrl(value)).toBe(SUPPORT_TELEGRAM_FALLBACK);
    }
  });
});
