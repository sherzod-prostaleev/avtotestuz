import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import PremiumPage from "./page";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <PremiumPage />
    </NextIntlClientProvider>
  );
}

describe("PremiumPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders premium header and features list", () => {
    renderWithIntl();
    expect(screen.getByText("DriveGo Premium")).toBeInTheDocument();
    expect(screen.getByText("Barcha 61 ta bilet cheklovsiz ochiladi")).toBeInTheDocument();
  });
});
