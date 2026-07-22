import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { CountdownTimer } from "./countdown-timer";

describe("CountdownTimer", () => {
  it("formats minutes and seconds with zero-padding", () => {
    render(<CountdownTimer remainingSeconds={125} />);
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("2:05");
  });

  it("uses the gold color when more than a minute remains", () => {
    render(<CountdownTimer remainingSeconds={90} />);
    expect(screen.getByTestId("countdown-timer").className).toContain("text-gold");
  });

  it("switches to a pulsating red state in the last minute", () => {
    render(<CountdownTimer remainingSeconds={45} />);
    const el = screen.getByTestId("countdown-timer");
    expect(el.className).toContain("text-danger");
    expect(el.className).toContain("animate-pulse");
  });

  it("never shows a negative time", () => {
    render(<CountdownTimer remainingSeconds={-5} />);
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("0:00");
  });
});
