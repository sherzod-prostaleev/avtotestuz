import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import ProfilePage from "./page";
import * as apiClient from "@/lib/api-client";
import { AUTOLOGIN_OFF_KEY, markTelegramMiniApp } from "@/lib/telegram/web-app";

const nav = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: nav.push, replace: nav.replace }),
  usePathname: () => "/uz-Latn/profile",
}));

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ProfilePage />
    </NextIntlClientProvider>
  );
}

describe("ProfilePage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    nav.push.mockReset();
    nav.replace.mockReset();
    sessionStorage.clear();
    delete (window as { Telegram?: unknown }).Telegram;
  });

  it("renders profile header and user info fields", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      profile: {
        id: "u1",
        phone: "+998901234567",
        name: "Sardor",
        region: "Toshkent",
        district: "",
        birth_date: null,
        locale_pref: "uz-Latn",
        theme_pref: "dark",
        referral_code: "ABC123",
        role: "user",
        created_at: "2026-07-22T00:00:00Z",
      },
      vip: { active: false, until: null },
    });

    renderWithIntl();

    expect(screen.getByText("Profil va Sozlamalar")).toBeInTheDocument();
    expect(screen.getByText("Shaxsiy ma'lumotlar")).toBeInTheDocument();
    expect(await screen.findByDisplayValue("Sardor")).toBeInTheDocument();
    expect(screen.getByDisplayValue("+998901234567")).toBeInTheDocument();
  });

  it("saves the editable fields using the PATCH /me contract", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      profile: {
        id: "u1",
        phone: "+998901234567",
        name: "Sardor",
        region: "Toshkent",
        district: "",
        birth_date: null,
        locale_pref: "uz-Latn",
        theme_pref: "dark",
        referral_code: "ABC123",
        role: "user",
        created_at: "2026-07-22T00:00:00Z",
      },
      vip: { active: false, until: null },
    });
    const patchSpy = vi.spyOn(apiClient, "apiPatch").mockResolvedValue({
      id: "u1",
      phone: "+998901234567",
      name: "Dilshod",
      region: "Samarqand",
    } as never);

    renderWithIntl();
    const nameInput = await screen.findByDisplayValue("Sardor");
    fireEvent.change(nameInput, { target: { value: "Dilshod" } });
    fireEvent.click(screen.getByRole("button", { name: "Saqlash" }));

    await waitFor(() =>
      expect(patchSpy).toHaveBeenCalledWith("me", { name: "Dilshod", region: "Toshkent" })
    );
  });

  // The phone list opens its sub-screens as panels, and only one is mounted at
  // a time. If they were all rendered and toggled with CSS, jsdom — which
  // applies none — would show six screens at once and every query by text
  // would find several of everything.
  it("opens one profile panel at a time on the phone", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      profile: {
        id: "u1",
        phone: "+998901234567",
        name: "Sardor",
        region: "Toshkent",
        district: "",
        birth_date: null,
        locale_pref: "uz-Latn",
        theme_pref: "dark",
        referral_code: "ABC123",
        role: "user",
        created_at: "2026-07-22T00:00:00Z",
      },
      vip: { active: true, until: "2026-12-31T00:00:00Z" },
    });

    renderWithIntl();

    // The list is showing: its rows are there, no panel is.
    const nameRow = await screen.findByRole("button", { name: /Ismingiz/ });
    expect(screen.getByRole("button", { name: /Parolni o'zgartirish/ })).toBeInTheDocument();
    // `personalInfo` is also the wide card's heading, which jsdom still shows,
    // so the panel is identified by the note only it renders.
    const panelNote = /Telefon raqamini o'zgartirib bo'lmaydi/;
    expect(screen.queryByText(panelNote)).not.toBeInTheDocument();

    fireEvent.click(nameRow);

    // Now the panel is showing and the list is gone — not merely hidden.
    expect(await screen.findByText(panelNote)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Parolni o'zgartirish/ })).not.toBeInTheDocument();
  });

  describe("logout", () => {
    const profileResponse = {
      profile: {
        id: "u1",
        phone: "+998901234567",
        name: "Sardor",
        region: "Toshkent",
        district: "",
        birth_date: null,
        locale_pref: "uz-Latn",
        theme_pref: "dark",
        referral_code: "ABC123",
        role: "user",
        created_at: "2026-07-22T00:00:00Z",
      },
      vip: { active: false, until: null },
    };

    it("signs out to the login screen on the website", async () => {
      vi.spyOn(apiClient, "apiGet").mockResolvedValue(profileResponse);
      const fetchMock = vi.fn(async () => new Response("{}", { status: 200 }));
      vi.stubGlobal("fetch", fetchMock);
      renderWithIntl();
      await screen.findByDisplayValue("Sardor");

      // Desktop card and phone list both render in jsdom; both use one handler.
      fireEvent.click(screen.getAllByRole("button", { name: /Tizimdan chiqish|Chiqish/ })[0]);

      await waitFor(() => expect(nav.push).toHaveBeenCalledWith("/uz-Latn/login"));
      expect(fetchMock).toHaveBeenCalledWith("/api/auth/logout", { method: "POST" });
      expect(nav.replace).not.toHaveBeenCalled();
    });

    // Spec D6: logout keeps the Telegram link and turns auto-login off
    // instead, then lands on /tg's welcome screen rather than /login.
    it("turns Telegram auto-login off first and lands on /tg inside the Mini App", async () => {
      markTelegramMiniApp();
      const order: string[] = [];
      const setItem = vi.fn((key: string, value: string, cb?: (err: string | null) => void) => {
        order.push(`cloud:${key}=${value}`);
        cb?.(null);
      });
      (window as { Telegram?: unknown }).Telegram = {
        WebApp: { initData: "x", CloudStorage: { setItem, getItem: vi.fn(), removeItem: vi.fn() } },
      };
      vi.spyOn(apiClient, "apiGet").mockResolvedValue(profileResponse);
      vi.stubGlobal(
        "fetch",
        vi.fn(async (input: RequestInfo | URL) => {
          order.push(`fetch:${String(input)}`);
          return new Response("{}", { status: 200 });
        }),
      );
      renderWithIntl();
      await screen.findByDisplayValue("Sardor");

      fireEvent.click(screen.getAllByRole("button", { name: /Tizimdan chiqish|Chiqish/ })[0]);

      await waitFor(() => expect(nav.replace).toHaveBeenCalledWith("/uz-Latn/tg"));
      expect(order).toEqual([`cloud:${AUTOLOGIN_OFF_KEY}=1`, "fetch:/api/auth/logout"]);
      expect(nav.push).not.toHaveBeenCalled();
    });
  });
});
