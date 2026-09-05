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

  // The load-bearing property of this component, and the reason it has a test
  // of its own: it asks once and then stops. A poll here resets the station
  // agent's idle clock, and the agent only installs a staged update after 30
  // minutes with no proxied API call -- so a heartbeat on this screen quietly
  // blocks agent updates for the whole fleet. Anyone reintroducing an interval
  // "so the banner refreshes itself" has to delete this test to do it.
  it("asks once and never polls, so the agent's idle clock can run", async () => {
    vi.useFakeTimers();
    const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: promo })));
    vi.stubGlobal("fetch", fetcher);
    render(wrap(<StationMobilePromo />));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(screen.getByTestId("mobile-promo-strip")).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(fetcher).toHaveBeenCalledWith("/api/proxy/me/mobile-promo", expect.anything());
    // Well past any plausible poll interval, and past the agent's 30-minute
    // idle window: still exactly one request.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(45 * 60 * 1000);
    });
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("mobile-promo-strip")).toBeInTheDocument();
  });

  it("shows nothing when the request fails, and asks again on the next mount", async () => {
    const fetcher = vi
      .fn()
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(new Response(JSON.stringify({ data: promo })));
    vi.stubGlobal("fetch", fetcher);
    const view = render(wrap(<StationMobilePromo />));
    await act(async () => {});
    expect(screen.queryByTestId("mobile-promo-strip")).not.toBeInTheDocument();
    expect(screen.queryByRole("complementary")).not.toBeInTheDocument();
    view.unmount();

    // Remounting is how the advert refreshes: a student returning to the home
    // screen, a reload, or the PC booting. That is the whole refresh story.
    render(wrap(<StationMobilePromo />));
    await act(async () => {});
    expect(await screen.findByTestId("mobile-promo-strip")).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
});
