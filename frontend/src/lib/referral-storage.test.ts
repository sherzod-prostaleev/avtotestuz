import { describe, it, expect, vi, beforeEach } from "vitest";
import * as apiClient from "@/lib/api-client";
import { ApiError } from "@/lib/api-client";
import {
  REFERRAL_CODE_STORAGE_KEY,
  applyPendingReferralCode,
  capturePendingReferralCodeFromUrl,
  readPendingReferralCode,
  storePendingReferralCode,
} from "./referral-storage";

function setSearch(search: string) {
  // jsdom allows replacing location.search only via a full URL assignment.
  window.history.replaceState({}, "", `/uz-Latn/dashboard${search}`);
}

describe("referral-storage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    window.localStorage.clear();
    setSearch("");
  });

  it("normalizes a captured code to upper case", () => {
    storePendingReferralCode("  ab12cd \n");
    expect(readPendingReferralCode()).toBe("AB12CD");
  });

  it("ignores a blank code", () => {
    storePendingReferralCode("   ");
    expect(readPendingReferralCode()).toBeNull();
  });

  it("captures ?ref from the current URL", () => {
    setSearch("?ref=friend01");
    capturePendingReferralCodeFromUrl();
    expect(readPendingReferralCode()).toBe("FRIEND01");
  });

  it("applies a stored code and clears it on success", async () => {
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({});
    storePendingReferralCode("ABC123");

    await applyPendingReferralCode();

    expect(post).toHaveBeenCalledWith("referral/apply", { code: "ABC123" });
    expect(readPendingReferralCode()).toBeNull();
  });

  it("does nothing when no code is stored", async () => {
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({});

    await applyPendingReferralCode();

    expect(post).not.toHaveBeenCalled();
  });

  // The regression this file exists for: the old flow deleted the code before
  // issuing the request, so one offline moment destroyed the referral for good.
  it("keeps the code after a transient failure so a later load can retry", async () => {
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(new Error("network down"));
    storePendingReferralCode("ABC123");

    await applyPendingReferralCode();

    expect(readPendingReferralCode()).toBe("ABC123");
  });

  it("keeps the code when the session is not established yet", async () => {
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(new ApiError("missing auth", "unauthorized", 401));
    storePendingReferralCode("ABC123");

    await applyPendingReferralCode();

    expect(readPendingReferralCode()).toBe("ABC123");
  });

  it.each([
    "referral_not_found",
    "referral_self",
    "referral_already_applied",
    "referral_not_eligible_paid",
    "referral_window_closed",
  ])(
    "drops the code when the server answers %s",
    async (code) => {
      vi.spyOn(apiClient, "apiPost").mockRejectedValue(new ApiError("nope", code, 400));
      storePendingReferralCode("ABC123");

      await applyPendingReferralCode();

      expect(window.localStorage.getItem(REFERRAL_CODE_STORAGE_KEY)).toBeNull();
    }
  );
});
