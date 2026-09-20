import { describe, expect, it } from "vitest";
import { canJudgeCardNetwork, cardDigits, detectCardNetwork } from "./card-network";

/**
 * This file exists to keep one client-side rule identical to
 * `billing.DetectCardNetwork` / `ValidatePayoutCard`. Where they disagree, the
 * server wins — and the client loses payouts it should have sent, which is
 * exactly what happened: the phone form demanded 8600 or 9860 exactly, so an
 * 86xx card the desktop form had already been paid out to was refused.
 */
describe("detectCardNetwork", () => {
  it("reads the two-digit prefix the endpoint reads", () => {
    expect(detectCardNetwork("8600123456789012")).toBe("uzcard");
    expect(detectCardNetwork("8617123456789012")).toBe("uzcard");
    expect(detectCardNetwork("9860123456789012")).toBe("humo");
    expect(detectCardNetwork("9863123456789012")).toBe("humo");
  });

  it("sees through the spaces people type", () => {
    expect(detectCardNetwork("8600 1234 5678 9012")).toBe("uzcard");
    expect(detectCardNetwork("9860-1234-5678-9012")).toBe("humo");
  });

  // Not a refusal: the endpoint takes such a card when told which network to
  // pay through, so the form asks instead of deciding for itself.
  it("places nothing it cannot place", () => {
    expect(detectCardNetwork("5614123456789012")).toBeNull();
    expect(detectCardNetwork("4278123456789012")).toBeNull();
  });

  it("waits for four digits before judging, as the endpoint does", () => {
    expect(detectCardNetwork("860")).toBeNull();
    expect(canJudgeCardNetwork("860")).toBe(false);
    expect(canJudgeCardNetwork("8600")).toBe(true);
    expect(detectCardNetwork("8600")).toBe("uzcard");
  });
});

describe("cardDigits", () => {
  it("keeps only digits", () => {
    expect(cardDigits(" 8600 1234-5678_9012 ")).toBe("8600123456789012");
    expect(cardDigits("")).toBe("");
  });
});
