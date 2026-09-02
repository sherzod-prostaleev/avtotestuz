import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import SavedPage from "./page";
import * as apiClient from "@/lib/api-client";
import { PROTECTED_SEGMENTS, matchesAny } from "@/lib/protected-segments";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <SavedPage />
    </NextIntlClientProvider>
  );
}

/** True once the locale prefix is stripped and every segment is checked
 * against the cookie gate — the same check src/proxy.ts runs on every
 * request from the login-free kiosk browser. */
function isKioskReachable(hrefOrPush: string): boolean {
  const withoutLocale = hrefOrPush.replace(/^\/[a-zA-Z-]+/, "");
  const pathname = withoutLocale.split("?")[0] || "/";
  return !matchesAny(pathname, PROTECTED_SEGMENTS);
}

const questionId = "11111111-1111-4111-8111-111111111111";
const savedDTO = [{ question_id: questionId, created_at: "2026-07-22T12:00:00Z" }];
/** The phone card names the category, so the page also asks for the list. */
const categoriesDTO = [{ code: "priority_intersections", name: "Chorrahalarda harakatlanish" }];
const questionDTO = {
  id: questionId,
  category_code: "priority_intersections",
  text: "Ushbu chorrahada qaysi transport vositasi birinchi o'tadi?",
  image_url: "https://media.example.test/question-1.webp",
  answers: [{ id: "secret-answer", position: 1, text: "Maxfiy to'g'ri javob", image_url: null }],
  correct_answer_id: "secret-answer",
};

describe("SavedPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads the real saved DTO then localized question details without correctness", async () => {
    vi.spyOn(apiClient, "apiGet")
      .mockResolvedValueOnce(savedDTO)
      .mockResolvedValueOnce(categoriesDTO)
      .mockResolvedValueOnce(questionDTO);

    renderWithIntl();

    // Two bodies render — the phone cards (`md:hidden`) and the wide grid
    // (`max-md:hidden`). jsdom applies no CSS, so both are in the DOM.
    expect(await screen.findAllByText(questionDTO.text)).toHaveLength(2);
    expect(apiClient.apiGet).toHaveBeenNthCalledWith(1, "me/saved");
    expect(apiClient.apiGet).toHaveBeenNthCalledWith(2, "categories?locale=uz-Latn");
    expect(apiClient.apiGet).toHaveBeenNthCalledWith(
      3,
      `questions/${questionId}?locale=uz-Latn`
    );
    // The wide grid still shows the raw code; the phone card shows the name.
    expect(screen.getByText("priority_intersections")).toBeInTheDocument();
    expect(screen.getByText("Chorrahalarda harakatlanish")).toBeInTheDocument();
    expect(screen.getAllByRole("img", { name: "1-saqlangan savol rasmi" })[1]).toHaveAttribute(
      "src",
      questionDTO.image_url
    );
    expect(screen.queryByText("Maxfiy to'g'ri javob")).not.toBeInTheDocument();
    expect(screen.queryByText("secret-answer")).not.toBeInTheDocument();
  });

  it("renders the honest empty state without requesting question details", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValueOnce([]);

    renderWithIntl();

    expect(await screen.findByText("Hali saqlangan savollar yo'q")).toBeInTheDocument();
    expect(apiClient.apiGet).toHaveBeenCalledTimes(1);
  });

  it("shows a load error and retries the complete request safely", async () => {
    vi.spyOn(apiClient, "apiGet")
      .mockRejectedValueOnce(new Error("network unavailable"))
      .mockResolvedValueOnce([]);

    renderWithIntl();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Saqlangan savollarni yuklab bo'lmadi."
    );
    fireEvent.click(screen.getByRole("button", { name: "Qayta urinish" }));

    expect(await screen.findByText("Hali saqlangan savollar yo'q")).toBeInTheDocument();
    expect(apiClient.apiGet).toHaveBeenCalledTimes(2);
  });

  it("keeps an item after a failed delete and removes it only after a successful retry", async () => {
    vi.spyOn(apiClient, "apiGet")
      .mockResolvedValueOnce(savedDTO)
      .mockResolvedValueOnce(categoriesDTO)
      .mockResolvedValueOnce(questionDTO);
    vi.spyOn(apiClient, "apiDelete")
      .mockRejectedValueOnce(new Error("delete failed"))
      .mockResolvedValueOnce({ ok: true });

    renderWithIntl();
    expect(await screen.findAllByText(questionDTO.text)).toHaveLength(2);

    // Both bodies expose a remove control with this name; either one works.
    fireEvent.click(screen.getAllByRole("button", { name: "Saqlanganlardan olib tashlash" })[0]);
    expect(await screen.findByText("Savolni olib tashlab bo'lmadi.")).toBeInTheDocument();
    expect(screen.getAllByText(questionDTO.text)).toHaveLength(2);

    fireEvent.click(screen.getAllByRole("button", { name: "Olib tashlashni qayta urinish" })[0]);

    await waitFor(() => expect(screen.queryByText(questionDTO.text)).not.toBeInTheDocument());
    expect(screen.getByText("Hali saqlangan savollar yo'q")).toBeInTheDocument();
    expect(apiClient.apiDelete).toHaveBeenCalledTimes(2);
    expect(apiClient.apiDelete).toHaveBeenNthCalledWith(1, `me/saved/${questionId}`);
    expect(apiClient.apiDelete).toHaveBeenNthCalledWith(2, `me/saved/${questionId}`);
  });
});

// Walks every navigation this page can perform for a cookie-less classroom
// kiosk browser (frontend/src/app/[locale]/(kiosk)/station/saved/page.tsx
// reuses this component with kiosk=true) and checks each destination against
// the same PROTECTED_SEGMENTS gate src/proxy.ts enforces — a kiosk browser
// carries no auth cookie, so a gated destination is a dead end at /login.
describe("SavedPage kiosk mode", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  function renderKiosk() {
    return render(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <SavedPage kiosk />
      </NextIntlClientProvider>
    );
  }

  it("keeps the back link under /station", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValueOnce([]);
    renderKiosk();

    const backLink = await screen.findByRole("link", { name: "Bosh sahifaga qaytish" });
    expect(backLink.getAttribute("href")).toBe("/uz-Latn/station");
    expect(isKioskReachable(backLink.getAttribute("href")!)).toBe(true);
  });

  it("sends the browse-tickets CTA (summary card and empty state) to a kiosk-reachable route", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValueOnce([]);
    renderKiosk();

    const ticketLinks = await screen.findAllByRole("link", { name: "Biletlarga o'tish" });
    expect(ticketLinks.length).toBeGreaterThanOrEqual(1);
    for (const link of ticketLinks) {
      const href = link.getAttribute("href") ?? "";
      expect(href).toBe("/uz-Latn/station/tickets");
      expect(isKioskReachable(href)).toBe(true);
    }
  });

  it("never renders a link into a protected segment, in any state", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValueOnce([]);
    renderKiosk();

    await screen.findByRole("link", { name: "Bosh sahifaga qaytish" });
    const hrefs = screen.getAllByRole("link").map((a) => a.getAttribute("href") ?? "");
    expect(hrefs.length).toBeGreaterThan(0);
    for (const href of hrefs) {
      expect(isKioskReachable(href)).toBe(true);
    }
    const withoutLocale = hrefs.map((h) => h.replace(/^\/[a-zA-Z-]+/, ""));
    expect(withoutLocale.some((h) => /^\/(dashboard|premium|checkout|profile)(\/|$|\?)/.test(h))).toBe(false);
  });

  it("offers no premium, checkout, profile or dashboard surface", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValueOnce([]);
    renderKiosk();

    await screen.findByRole("link", { name: "Bosh sahifaga qaytish" });
    expect(screen.queryByRole("link", { name: /premium/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /checkout/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /profile/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /dashboard/i })).not.toBeInTheDocument();
  });
});
