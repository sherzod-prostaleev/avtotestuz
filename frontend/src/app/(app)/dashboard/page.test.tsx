import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import DashboardPage from "./page";

describe("DashboardPage", () => {
  it("renders the streak count and readiness ring", () => {
    render(<DashboardPage />);
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("68%")).toBeInTheDocument();
  });

  it("renders all four navigation cards", () => {
    render(<DashboardPage />);
    expect(screen.getByText("Biletlar")).toBeInTheDocument();
    expect(screen.getByText("Imtihon simulyatsiyasi")).toBeInTheDocument();
    expect(screen.getByText("Mashq")).toBeInTheDocument();
    expect(screen.getByText("Xatolar ustida ishlash")).toBeInTheDocument();
  });

  it("shows the free-plan badge when the user is not VIP", () => {
    render(<DashboardPage />);
    expect(screen.getByText("Bepul reja")).toBeInTheDocument();
  });
});
