import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi } from "vitest";
import messages from "../../../../messages/uz-Latn.json";
import LandingPage from "./page";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  usePathname: () => "/uz-Latn",
}));

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <LandingPage />
    </NextIntlClientProvider>
  );
}

describe("LandingPage", () => {
  it("renders the hero CTA and all proof stats", () => {
    renderWithIntl();
    expect(screen.getAllByRole("button", { name: /Bepul/i }).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("1235")).toBeInTheDocument();
    expect(screen.getByText("61")).toBeInTheDocument();
  });

  it("renders the interactive demo question", () => {
    renderWithIntl();
    expect(screen.getByText(/O'ngdan keluvchi haydovchi/)).toBeInTheDocument();
  });
});
