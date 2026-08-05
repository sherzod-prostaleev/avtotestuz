import { describe, expect, it } from "vitest";
import uzLatn from "../../messages/uz-Latn.json";
import uzCyrl from "../../messages/uz-Cyrl.json";
import ru from "../../messages/ru.json";

describe("B2B locale contract", () => {
  it("keeps AdminB2B keys identical in all three locales", () => {
    for (const section of ["AdminB2B"] as const) {
      const expected = Object.keys(uzLatn[section]).sort();
      expect(Object.keys(uzCyrl[section]).sort()).toEqual(expected);
      expect(Object.keys(ru[section]).sort()).toEqual(expected);
    }
  });

  it("contains the destructive confirmations and multi-PC explanation", () => {
    for (const messages of [uzLatn, uzCyrl, ru]) {
      expect(messages.AdminUsers.deleteButton).toBeTruthy();
      expect(messages.AdminUsers.deletePhrase).toBeTruthy();
      expect(messages.AdminB2B.deleteButton).toBeTruthy();
      expect(messages.AdminB2B.multiPcStepLearner).toBeTruthy();
    }
  });
});
