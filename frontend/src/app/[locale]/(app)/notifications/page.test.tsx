import { render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import NotificationsPage from "./page";
import * as client from "@/lib/notifications-client";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

function minutesAgo(minutes: number): string {
  return new Date(Date.now() - minutes * 60_000).toISOString();
}

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <NotificationsPage />
    </NextIntlClientProvider>
  );
}

// The page renders two lists — the icon-led phone one (`md:hidden`) and the
// wide one (`max-md:hidden`). jsdom applies no CSS, so both are in the DOM and
// every title appears twice.
describe("NotificationsPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders each notification in both the phone and the wide list", async () => {
    vi.spyOn(client, "listNotifications").mockResolvedValue([
      {
        id: "n1",
        title: "14 ta savol takrorlashga tayyor",
        body: "Xatolar banki bugungi navbatni ochdi",
        read_at: null,
        created_at: minutesAgo(10),
        url: "/uz-Latn/mistakes",
      },
    ]);

    renderWithIntl();

    expect(await screen.findAllByText("14 ta savol takrorlashga tayyor")).toHaveLength(2);
  });

  it("dates the phone rows relatively, not absolutely", async () => {
    vi.spyOn(client, "listNotifications").mockResolvedValue([
      { id: "a", title: "A", body: "", read_at: null, created_at: minutesAgo(10) },
      { id: "b", title: "B", body: "", read_at: null, created_at: minutesAgo(3 * 60) },
      { id: "c", title: "C", body: "", read_at: null, created_at: minutesAgo(26 * 60) },
      { id: "d", title: "D", body: "", read_at: null, created_at: minutesAgo(3 * 24 * 60) },
    ]);

    renderWithIntl();

    expect(await screen.findByText("10 daq")).toBeInTheDocument();
    expect(screen.getByText("3 soat")).toBeInTheDocument();
    expect(screen.getByText("kecha")).toBeInTheDocument();
    expect(screen.getByText("3 kun")).toBeInTheDocument();
  });

  // The API returns no `type`, so the phone row's icon is read off the
  // destination route. A notification without a `url`, or with one pointing
  // somewhere unmapped, must still render a row rather than blow up.
  it("falls back to a neutral icon when the destination says nothing", async () => {
    vi.spyOn(client, "listNotifications").mockResolvedValue([
      { id: "n1", title: "Chegirma", body: "", read_at: null, created_at: minutesAgo(5) },
      {
        id: "n2",
        title: "Nomaʼlum",
        body: "",
        read_at: null,
        created_at: minutesAgo(5),
        url: "/uz-Latn/hech-qayerda",
      },
    ]);

    renderWithIntl();

    await waitFor(() => expect(screen.getAllByText("Chegirma")).toHaveLength(2));
    expect(screen.getAllByText("Nomaʼlum")).toHaveLength(2);
  });

  it("shows the empty state when there is nothing to read", async () => {
    vi.spyOn(client, "listNotifications").mockResolvedValue([]);

    renderWithIntl();

    expect(await screen.findByText(messages.Notifications.empty)).toBeInTheDocument();
  });
});
