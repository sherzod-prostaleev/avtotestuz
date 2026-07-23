import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import ProfilePage from "./page";
import * as apiClient from "@/lib/api-client";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
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
});
