import { render, screen } from "@testing-library/react";
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
      id: "u1",
      phone: "+998901234567",
      name: "Sardor",
      region: "Toshkent",
    } as any);

    renderWithIntl();

    expect(screen.getByText("Profil va Sozlamalar")).toBeInTheDocument();
    expect(screen.getByText("Shaxsiy ma'lumotlar")).toBeInTheDocument();
  });
});
