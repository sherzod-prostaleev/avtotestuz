import { afterEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import messages from "../../../messages/uz-Latn.json";
import { MobilePromoBanner, StationMobilePromo } from "./mobile-promo";

const promo = {
  enabled: true,
  url: "https://drivergo.uz/r/REF-C62LC2",
  qr_data_url: "data:image/png;base64,dGVzdA==",
};
const wrap = (child: React.ReactNode) => (
  <NextIntlClientProvider locale="uz-Latn" messages={messages}>
    {child}
  </NextIntlClientProvider>
);
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("station mobile promotion", () => {
  it("renders a QR without a kiosk navigation link and hides disabled banners", () => {
    const { rerender } = render(wrap(<MobilePromoBanner promo={promo} />));
    expect(screen.getByRole("img", { name: messages.MobilePromo.qrAlt })).toHaveAttribute(
      "src",
      promo.qr_data_url,
    );
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    rerender(wrap(<MobilePromoBanner promo={{ ...promo, enabled: false }} />));
    expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
  });

  // The banner alone sat at y=816 on every classroom resolution we ship to, so
  // a student who never scrolled never saw the advert. The pinned strip is the
  // part that is actually on screen; losing it would silently undo the fix.
  it("pins a second QR to the bottom of the kiosk screen", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: promo }))),
    );
    render(wrap(<StationMobilePromo />));
    const strip = await screen.findByTestId("mobile-promo-strip");
    expect(strip.className).toContain("fixed");
    expect(strip.className).toContain("bottom-0");
    // Same QR as the banner, and silent to assistive tech: it is a duplicate.
    expect(strip.querySelector("img")).toHaveAttribute("src", promo.qr_data_url);
    expect(strip).toHaveAttribute("aria-hidden", "true");
  });

  // A school that switched the advert off must get the station screen exactly
  // as it was before this feature existed -- no strip, no reserved space.
  it("renders nothing at all while the school has the advert switched off", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { enabled: false, url: "" } }))),
    );
    const { container } = render(wrap(<StationMobilePromo />));
    await act(async () => {});
    expect(screen.queryByTestId("mobile-promo-strip")).not.toBeInTheDocument();
    expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
    expect(container).toBeEmptyDOMElement();
  });

  it.each(["disabled", "offline"])("removes the advert when the next poll is %s", async (mode) => {
    vi.useFakeTimers();
    const fetcher = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ data: promo })));
    if (mode === "disabled")
      fetcher.mockResolvedValueOnce(new Response(JSON.stringify({ data: { enabled: false, url: "" } })));
    else fetcher.mockRejectedValueOnce(new Error("offline"));
    vi.stubGlobal("fetch", fetcher);
    const view = render(wrap(<StationMobilePromo />));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(screen.getByRole("complementary")).toBeInTheDocument();
    expect(screen.getByTestId("mobile-promo-strip")).toBeInTheDocument();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(75000);
    });
    expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
    expect(screen.queryByTestId("mobile-promo-strip")).not.toBeInTheDocument();
    view.unmount();
    await vi.advanceTimersByTimeAsync(150000);
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
});
