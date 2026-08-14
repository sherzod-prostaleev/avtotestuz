import { describe, expect, it } from "vitest";
import {
  formatNationalPhone,
  normalizeNationalPhone,
  parsePasswordResetTokenFromBotURL,
} from "./phone-format";

describe("normalizeNationalPhone", () => {
  it("keeps 9 national digits and strips 998", () => {
    expect(normalizeNationalPhone("90 111 22 33")).toBe("901112233");
    expect(normalizeNationalPhone("+998901112233")).toBe("901112233");
    expect(normalizeNationalPhone("998901112233")).toBe("901112233");
  });
});

describe("formatNationalPhone", () => {
  it("groups digits as 90 123 45 67", () => {
    expect(formatNationalPhone("901234567")).toBe("90 123 45 67");
    expect(formatNationalPhone("90")).toBe("90");
    expect(formatNationalPhone("90123")).toBe("90 123");
  });
});

describe("parsePasswordResetTokenFromBotURL", () => {
  it("reads the pwr_ start payload", () => {
    expect(parsePasswordResetTokenFromBotURL("https://t.me/AvtoTestBot?start=pwr_abc")).toBe("abc");
    expect(parsePasswordResetTokenFromBotURL("https://t.me/AvtoTestBot?start=linktok")).toBeNull();
    expect(parsePasswordResetTokenFromBotURL("not a url")).toBeNull();
  });
});
