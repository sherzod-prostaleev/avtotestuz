import { describe, expect, it } from "vitest";
import {
  emptyLegalBundle,
  legalBodyOrEmpty,
  normalizeLegalBundle,
  parseLegalBody,
} from "./site-legal";

describe("site-legal", () => {
  it("normalizes partial bundles", () => {
    const got = normalizeLegalBundle({
      locales: { "uz-Latn": { oferta: "  Hi  ", privacy: "P" }, ru: { oferta: "RU" } },
    });
    expect(got.locales["uz-Latn"].oferta).toBe("Hi");
    expect(got.locales["uz-Latn"].privacy).toBe("P");
    expect(got.locales.ru.oferta).toBe("RU");
    expect(got.locales["uz-Cyrl"].oferta).toBe("");
  });

  it("emptyLegalBundle has all locales", () => {
    const b = emptyLegalBundle();
    expect(Object.keys(b.locales).sort()).toEqual(["ru", "uz-Cyrl", "uz-Latn"]);
  });

  it("legalBodyOrEmpty trims", () => {
    expect(legalBodyOrEmpty("  x  ")).toBe("x");
    expect(legalBodyOrEmpty("   ")).toBe("");
  });

  it("parseLegalBody splits paragraphs and ## headings", () => {
    const blocks = parseLegalBody("## Kirish\n\nBirinchi paragraf.\n\nIkkinchi.");
    expect(blocks).toEqual([
      { type: "h2", text: "Kirish" },
      { type: "p", text: "Birinchi paragraf." },
      { type: "p", text: "Ikkinchi." },
    ]);
  });
});
