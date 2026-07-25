import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { certificateShareUrl, shareOrCopyCertificateLink } from "./certificate-share";

describe("certificateShareUrl", () => {
  it("joins origin locale and code", () => {
    expect(certificateShareUrl("https://drivergo.uz/", "uz-Latn", "abc")).toBe(
      "https://drivergo.uz/uz-Latn/sertifikat/abc",
    );
  });
});

describe("shareOrCopyCertificateLink", () => {
  beforeEach(() => {
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("uses clipboard when share unavailable", async () => {
    // @ts-expect-error test stub
    delete navigator.share;
    const result = await shareOrCopyCertificateLink({
      url: "https://x/y",
      title: "t",
      text: "b",
    });
    expect(result).toBe("clipboard");
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith("https://x/y");
  });

  it("prefers navigator.share when present", async () => {
    const share = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { share });
    const result = await shareOrCopyCertificateLink({
      url: "https://x/y",
      title: "t",
      text: "b",
    });
    expect(result).toBe("share");
    expect(share).toHaveBeenCalled();
  });
});
