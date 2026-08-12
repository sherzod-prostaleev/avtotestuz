import { describe, expect, it } from "vitest";
import {
  ADMIN_PAYMENTS_DEFAULT_STATUS,
  isAdminPaymentVoidable,
} from "./admin-payment-voidable";

describe("isAdminPaymentVoidable", () => {
  it("allows void on failed and open manual checkouts only", () => {
    expect(isAdminPaymentVoidable({ status: "failed", provider: "payme" })).toBe(true);
    expect(isAdminPaymentVoidable({ status: "created", provider: "manual" })).toBe(true);
    expect(isAdminPaymentVoidable({ status: "pending", provider: "manual" })).toBe(true);
  });

  it("hides void on canceled, paid, and in-flight provider checkouts", () => {
    expect(isAdminPaymentVoidable({ status: "canceled", provider: "manual" })).toBe(false);
    expect(isAdminPaymentVoidable({ status: "canceled", provider: "click" })).toBe(false);
    expect(isAdminPaymentVoidable({ status: "paid", provider: "manual" })).toBe(false);
    expect(isAdminPaymentVoidable({ status: "created", provider: "click" })).toBe(false);
  });
});

describe("ADMIN_PAYMENTS_DEFAULT_STATUS", () => {
  it("defaults the directory to paid so abandoned checkouts are not the home view", () => {
    expect(ADMIN_PAYMENTS_DEFAULT_STATUS).toBe("paid");
  });
});
