import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import MistakesPage from "./page";
import * as apiClient from "@/lib/api-client";

const push = vi.fn();

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace: vi.fn() }),
}));

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <MistakesPage />
    </NextIntlClientProvider>
  );
}

describe("MistakesPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    push.mockReset();
  });

  it("renders the exact due/total contract and starts a due session", async () => {
    vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
      if (path === "me/mistakes") {
        return { due_count: 5, total_bank_count: 12, next_due_at: null } as never;
      }
      if (path === "me/entitlement") {
        return { active: true, until: "2026-08-01T00:00:00Z" } as never;
      }
      throw new Error(`unexpected path: ${path}`);
    });

    renderWithIntl();

    // Two bodies render — the wide one and the phone one (`md:hidden`). jsdom
    // applies no CSS, so both headings and both copies of each figure are in
    // the DOM at once.
    expect(screen.getByText("Xatolar ustida ishlash")).toBeInTheDocument();
    expect(screen.getByText("Xatolar banki")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getAllByText("5")).toHaveLength(2);
      expect(screen.getAllByText("12")).toHaveLength(2);
    });

    expect(apiClient.apiGet).toHaveBeenCalledWith("me/mistakes");
    expect(apiClient.apiGet).toHaveBeenCalledWith("me/entitlement");
    // Both CTAs share the name; clicking either must start the same session.
    fireEvent.click(screen.getAllByRole("button", { name: "Xatolarni takrorlash" })[0]);
    expect(push).toHaveBeenCalledWith("/uz-Latn/session/start?mode=mistakes&count=5");
  });

  it("distinguishes a genuinely empty bank from nothing due today", async () => {
    const get = vi.spyOn(apiClient, "apiGet");
    get.mockImplementation(async (path: string) => {
      if (path === "me/mistakes") {
        return { due_count: 0, total_bank_count: 0, next_due_at: null } as never;
      }
      return { active: true, until: null } as never;
    });

    const view = renderWithIntl();
    expect(await screen.findByText("Xatolar banki hali bo'sh")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Biletlarni ochish" })).toHaveAttribute(
      "href",
      "/uz-Latn/tickets"
    );

    get.mockImplementation(async (path: string) => {
      if (path === "me/mistakes") {
        return {
          due_count: 0,
          total_bank_count: 3,
          next_due_at: "2026-07-23T08:00:00Z",
        } as never;
      }
      return { active: true, until: null } as never;
    });
    view.unmount();
    renderWithIntl();

    expect(await screen.findByText("Ajoyib — hozir takrorlash shart emas")).toBeInTheDocument();
    expect(screen.getByText(/23.07.26/)).toBeInTheDocument();
  });

  it("shows the premium path for a free profile without hiding real counters", async () => {
    vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
      if (path === "me/mistakes") {
        return { due_count: 2, total_bank_count: 7, next_due_at: null } as never;
      }
      return { active: false, until: null } as never;
    });

    renderWithIntl();

    expect(await screen.findByText("Xatolar banki Premium obunachilar uchun")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("7")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Premiumni ko'rish" })).toHaveAttribute(
      "href",
      "/uz-Latn/premium"
    );
  });

  it("uses a localized recoverable error and retries the whole request", async () => {
    const get = vi
      .spyOn(apiClient, "apiGet")
      .mockRejectedValueOnce(new apiClient.ApiError("database exploded", "internal", 500))
      .mockResolvedValueOnce({ active: true, until: null } as never)
      .mockImplementation(async (path: string) => {
        if (path === "me/mistakes") {
          return { due_count: 1, total_bank_count: 1, next_due_at: null } as never;
        }
        return { active: true, until: null } as never;
      });

    renderWithIntl();

    expect(await screen.findByText("Xatolar bankini yuklab bo'lmadi.")).toBeInTheDocument();
    expect(screen.queryByText("database exploded")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Qayta urinish" }));
    // Two in the wide grid, two more in the phone card and its info row.
    await waitFor(() => expect(screen.getAllByText("1")).toHaveLength(4));
    expect(get.mock.calls.length).toBeGreaterThanOrEqual(4);
  });
});
