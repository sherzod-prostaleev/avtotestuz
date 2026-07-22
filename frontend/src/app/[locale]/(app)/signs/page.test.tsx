import { render, screen, fireEvent } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import SignsPage from "./page";
import * as useSignsModule from "@/hooks/use-signs";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  usePathname: () => "/uz-Latn/signs",
}));

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <SignsPage />
    </NextIntlClientProvider>
  );
}

describe("SignsPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders signs catalog header and sign cards", () => {
    vi.spyOn(useSignsModule, "useSigns").mockReturnValue({
      signs: [
        {
          id: "s1",
          code: "3.27",
          group_code: "prohibitory",
          group_name: "Taqiqlovchi",
          name: "To'xtash taqiqlangan",
          description: "Transport vositalarining to'xtashi taqiqlanadi",
          image_url: "/signs/3.27.png",
        },
      ],
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    renderWithIntl();

    expect(screen.getByText("Yo'l belgilari katalogi")).toBeInTheDocument();
    expect(screen.getByText("3.27")).toBeInTheDocument();
    expect(screen.getByText("To'xtash taqiqlangan")).toBeInTheDocument();
  });

  it("opens details modal when sign card is clicked", () => {
    vi.spyOn(useSignsModule, "useSigns").mockReturnValue({
      signs: [
        {
          id: "s1",
          code: "3.27",
          group_code: "prohibitory",
          group_name: "Taqiqlovchi",
          name: "To'xtash taqiqlangan",
          description: "Transport vositalarining to'xtashi taqiqlanadi",
          image_url: "/signs/3.27.png",
        },
      ],
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    renderWithIntl();

    const signCard = screen.getByText("To'xtash taqiqlangan");
    fireEvent.click(signCard);

    expect(screen.getByText("Transport vositalarining to'xtashi taqiqlanadi")).toBeInTheDocument();
    expect(screen.getByText("Yopish")).toBeInTheDocument();
  });
});
