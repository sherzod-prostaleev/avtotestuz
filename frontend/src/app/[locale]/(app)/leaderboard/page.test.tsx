import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import LeaderboardPage, { type LeaderboardResponse } from "./page";
import * as apiClient from "@/lib/api-client";
import { PROTECTED_SEGMENTS, matchesAny } from "@/lib/protected-segments";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

function makeResponse(overrides: Partial<LeaderboardResponse> = {}): LeaderboardResponse {
  return {
    period: "weekly",
    you: { rank: 42, score: 87, name: "Foydalanuvchi #a3f1" },
    top: [
      { rank: 1, name: "Aziz Karimov", score: 340 },
      { rank: 2, name: "Foydalanuvchi #9c02", score: 312 },
    ],
    around_you: [],
    ...overrides,
  };
}

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <LeaderboardPage />
    </NextIntlClientProvider>
  );
}

describe("LeaderboardPage", () => {
  // The page renders two bodies — the phone one (`md:hidden`) and the wide one
  // (`max-md:hidden`). jsdom applies no CSS, so both are in the DOM and every
  // shared string appears twice; `*AllBy*` is the honest query here.
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads the weekly period by default and renders your rank plus the top list", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue(makeResponse());

    renderWithIntl();

    expect(await screen.findAllByText("Aziz Karimov")).toHaveLength(2);
    expect(apiClient.apiGet).toHaveBeenCalledWith("leaderboard?period=weekly");
    expect(screen.getAllByText("#42")).toHaveLength(2);
    // The wide card shows the score on its own; the phone card folds it into a
    // sentence, so this number is deliberately not duplicated.
    expect(screen.getByText("87")).toBeInTheDocument();
    expect(screen.getByText("87 ball · bu hafta")).toBeInTheDocument();
    expect(screen.getAllByText("340")).toHaveLength(2);
    // Around-you is empty here, so that section must not render at all.
    expect(screen.queryByText("Reytingdagi qo'shnilaringiz")).not.toBeInTheDocument();
  });

  it("switches periods when a tab is clicked and refetches", async () => {
    const get = vi.spyOn(apiClient, "apiGet").mockResolvedValue(makeResponse());
    renderWithIntl();
    await screen.findAllByText("Aziz Karimov");

    fireEvent.click(screen.getByRole("radio", { name: "Kun" }));

    await waitFor(() => expect(get).toHaveBeenCalledWith("leaderboard?period=daily"));
    expect(screen.getByRole("radio", { name: "Kun" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("radio", { name: "Hafta" })).toHaveAttribute("aria-checked", "false");
  });

  it("shows the around-you section only when the backend returns neighbors", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue(
      makeResponse({
        you: { rank: 42, score: 87, name: "Foydalanuvchi #a3f1" },
        around_you: [
          { rank: 41, name: "Foydalanuvchi #1111", score: 88 },
          { rank: 42, name: "Foydalanuvchi #a3f1", score: 87 },
          { rank: 43, name: "Foydalanuvchi #2222", score: 85 },
        ],
      })
    );

    renderWithIntl();

    expect(await screen.findByText("Reytingdagi qo'shnilaringiz")).toBeInTheDocument();
    // The neighbor row matching the caller's own rank is tagged "Siz".
    expect(screen.getByText("Siz")).toBeInTheDocument();
  });

  it("shows a not-ranked hint when the profile has no score yet this period", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue(
      makeResponse({ you: { rank: null, score: 0, name: "Foydalanuvchi #a3f1" } })
    );

    renderWithIntl();

    expect(await screen.findAllByText("—")).toHaveLength(2);
    expect(
      screen.getAllByText("Siz bu davrda hali ball to'plamadingiz. Reytingga kirish uchun to'g'ri javob bering!")
    ).toHaveLength(2);
  });

  it("renders an empty state when nobody has scored in the period yet", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue(
      makeResponse({ top: [], you: { rank: null, score: 0, name: "Foydalanuvchi #a3f1" } })
    );

    renderWithIntl();

    expect(
      await screen.findAllByText("Bu davrda hali hech kim ball to'plamadi. Birinchi bo'ling!")
    ).toHaveLength(2);
  });

  it("shows a loading state, then an error state with a working retry", async () => {
    const get = vi.spyOn(apiClient, "apiGet").mockRejectedValue(new Error("network down"));

    renderWithIntl();

    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(await screen.findByRole("alert")).toHaveTextContent("Reytingni yuklab bo'lmadi.");

    get.mockResolvedValue(makeResponse());
    fireEvent.click(screen.getByRole("button", { name: "Qayta urinib ko'ring" }));

    expect(await screen.findAllByText("Aziz Karimov")).toHaveLength(2);
  });
});

/** True once the locale prefix is stripped and every segment is checked
 * against the cookie gate — the same check src/proxy.ts runs on every
 * request from the login-free kiosk browser. */
function isKioskReachable(hrefOrPush: string): boolean {
  const withoutLocale = hrefOrPush.replace(/^\/[a-zA-Z-]+/, "");
  const pathname = withoutLocale.split("?")[0] || "/";
  return !matchesAny(pathname, PROTECTED_SEGMENTS);
}

// Walks every navigation this page can perform for a cookie-less classroom
// kiosk browser (frontend/src/app/[locale]/(kiosk)/station/leaderboard/page.tsx
// reuses this component with kiosk=true) and checks each destination against
// the same PROTECTED_SEGMENTS gate src/proxy.ts enforces — a kiosk browser
// carries no auth cookie, so a gated destination is a dead end at /login.
describe("LeaderboardPage kiosk mode", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  function renderKiosk(overrides: Partial<LeaderboardResponse> = {}) {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue(makeResponse(overrides));
    return render(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <LeaderboardPage kiosk />
      </NextIntlClientProvider>
    );
  }

  it("keeps the back link under /station", async () => {
    renderKiosk();

    const backLink = await screen.findByRole("link", { name: /Bosh sahifaga qaytish/ });
    expect(backLink.getAttribute("href")).toBe("/uz-Latn/station");
    expect(isKioskReachable(backLink.getAttribute("href")!)).toBe(true);
  });

  it("never renders a link into a protected segment, in any state", async () => {
    renderKiosk();

    await screen.findAllByText("Aziz Karimov");
    const hrefs = screen.getAllByRole("link").map((a) => a.getAttribute("href") ?? "");
    expect(hrefs.length).toBeGreaterThan(0);
    for (const href of hrefs) {
      expect(isKioskReachable(href)).toBe(true);
    }
  });

  it("offers no premium, checkout, profile or dashboard surface", async () => {
    renderKiosk();

    await screen.findAllByText("Aziz Karimov");
    expect(screen.queryByRole("link", { name: /premium/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /checkout/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /profile/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /dashboard/i })).not.toBeInTheDocument();
  });

  // A station never accumulates points (leaderboard.Service.RecordPoint
  // returns early for a kind='station' profile), so a kiosk caller always
  // has you.rank === null. The board itself must still render cleanly.
  it("renders without error when the caller has no rank", async () => {
    renderKiosk({ you: { rank: null, score: 0, name: "PC-1" } });

    expect(await screen.findAllByText("Aziz Karimov")).toHaveLength(2);
    expect(screen.getAllByText("—")).toHaveLength(2);
    expect(
      screen.getAllByText("Siz bu davrda hali ball to'plamadingiz. Reytingga kirish uchun to'g'ri javob bering!")
    ).toHaveLength(2);
  });
});
