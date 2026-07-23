import { act, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import { TrialCountdown } from "./trial-countdown";

const NOW = new Date("2026-07-23T12:00:00.000Z");

function renderCountdown(props: React.ComponentProps<typeof TrialCountdown>) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <TrialCountdown {...props} />
    </NextIntlClientProvider>
  );
}

function inHours(hours: number): string {
  return new Date(NOW.getTime() + hours * 3600_000).toISOString();
}

describe("TrialCountdown", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders nothing without an active entitlement", () => {
    const { container } = renderCountdown({ isVip: false, validUntil: inHours(5) });
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when the entitlement has no end date", () => {
    const { container } = renderCountdown({ isVip: true, validUntil: null });
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the remaining time as hours, minutes and seconds", () => {
    renderCountdown({ isVip: true, validUntil: inHours(23.5) });
    expect(screen.getByTestId("trial-countdown")).toHaveTextContent("23:30:00");
  });

  it("counts down as real time passes", () => {
    renderCountdown({ isVip: true, validUntil: inHours(1) });
    expect(screen.getByTestId("trial-countdown")).toHaveTextContent("01:00:00");

    act(() => {
      vi.advanceTimersByTime(65_000);
    });
    expect(screen.getByTestId("trial-countdown")).toHaveTextContent("00:58:55");
  });

  // A laptop that sleeps for an hour must not come back an hour behind: the
  // remaining time is derived from the expiry timestamp, never decremented.
  it("stays accurate after the tab is suspended", () => {
    renderCountdown({ isVip: true, validUntil: inHours(5) });

    act(() => {
      vi.setSystemTime(new Date(NOW.getTime() + 4 * 3600_000));
      vi.advanceTimersByTime(1000);
    });
    expect(screen.getByTestId("trial-countdown")).toHaveTextContent("00:59:59");
  });

  it("switches to an upgrade prompt once the trial has run out", () => {
    renderCountdown({ isVip: true, validUntil: inHours(-1) });

    expect(screen.queryByTestId("trial-countdown")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Premium olish" })).toBeInTheDocument();
  });

  it("turns into an upgrade prompt when the countdown reaches zero live", () => {
    renderCountdown({ isVip: true, validUntil: inHours(0.001) });

    act(() => {
      vi.advanceTimersByTime(10_000);
    });
    expect(screen.queryByTestId("trial-countdown")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Premium olish" })).toBeInTheDocument();
  });
});
