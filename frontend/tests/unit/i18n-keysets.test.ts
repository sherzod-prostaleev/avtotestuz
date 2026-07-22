import { describe, expect, it } from "vitest";
import uzLatnMessages from "../../messages/uz-Latn.json";
import uzCyrlMessages from "../../messages/uz-Cyrl.json";
import ruMessages from "../../messages/ru.json";

const messageSets = [uzLatnMessages, uzCyrlMessages, ruMessages];

describe("localized message contracts", () => {
  it("keeps top-level namespaces identical", () => {
    const expected = Object.keys(uzLatnMessages).sort();
    for (const messages of messageSets.slice(1)) {
      expect(Object.keys(messages).sort()).toEqual(expected);
    }
  });

  it("keeps every namespace's keys identical in all locales", () => {
    for (const namespace of Object.keys(uzLatnMessages) as Array<keyof typeof uzLatnMessages>) {
      const expected = Object.keys(uzLatnMessages[namespace]).sort();
      for (const messages of messageSets.slice(1)) {
        expect(Object.keys(messages[namespace]).sort()).toEqual(expected);
      }
    }
  });
});
