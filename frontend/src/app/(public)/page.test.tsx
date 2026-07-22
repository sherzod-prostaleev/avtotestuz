import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import LandingPage from "./page";

describe("LandingPage", () => {
  it("renders the hero CTA and all proof stats", () => {
    render(<LandingPage />);
    expect(screen.getByRole("button", { name: "Bepul boshlash" })).toBeInTheDocument();
    expect(screen.getByText("1235")).toBeInTheDocument();
    expect(screen.getByText("61")).toBeInTheDocument();
  });

  it("renders the interactive demo question", () => {
    render(<LandingPage />);
    expect(screen.getByText(/svetofor ishlamayapti/)).toBeInTheDocument();
  });
});
