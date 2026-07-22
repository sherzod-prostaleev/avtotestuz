import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect } from "vitest";
import messages from "../../../../messages/uz-Latn.json";
import LandingPage from "./page";

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
    expect(screen.getByRole("button", { name: "Bepul boshlash" })).toBeInTheDocument();
    expect(screen.getByText("1235")).toBeInTheDocument();
    expect(screen.getByText("61")).toBeInTheDocument();
  });

  it("renders the interactive demo question", () => {
    renderWithIntl();
    expect(screen.getByText(/svetofor ishlamayapti/)).toBeInTheDocument();
  });
});
