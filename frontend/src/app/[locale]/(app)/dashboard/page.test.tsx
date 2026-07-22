import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import DashboardPage from "./page";

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <DashboardPage />
    </NextIntlClientProvider>
  );
}

describe("DashboardPage", () => {
  it("renders the streak count and readiness ring", () => {
    renderWithIntl();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("68%")).toBeInTheDocument();
  });

  it("renders all four navigation cards", () => {
    renderWithIntl();
    expect(screen.getByText("Biletlar")).toBeInTheDocument();
    expect(screen.getByText("Imtihon simulyatsiyasi")).toBeInTheDocument();
    expect(screen.getByText("Mashq")).toBeInTheDocument();
    expect(screen.getByText("Xatolar ustida ishlash")).toBeInTheDocument();
  });

  it("shows the free-plan badge when the user is not VIP", () => {
    renderWithIntl();
    expect(screen.getByText("Bepul reja")).toBeInTheDocument();
  });
});
