import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ResultRing } from "./result-ring";

describe("ResultRing", () => {
  it("renders the clamped percentage as text", () => {
    render(<ResultRing percent={68} label="Tayyorlik" />);
    expect(screen.getByText("68%")).toBeInTheDocument();
    expect(screen.getByText("Tayyorlik")).toBeInTheDocument();
  });

  it("uses the gold ring color at 80% and above", () => {
    render(<ResultRing percent={85} />);
    expect(screen.getByTestId("result-ring-progress").getAttribute("class")).toContain("text-gold");
  });

  it("uses the danger ring color below 50%", () => {
    render(<ResultRing percent={30} />);
    expect(screen.getByTestId("result-ring-progress").getAttribute("class")).toContain("text-danger");
  });

  it("computes a full stroke offset (no progress) at 0%", () => {
    render(<ResultRing percent={0} />);
    const circle = screen.getByTestId("result-ring-progress");
    expect(circle.getAttribute("stroke-dashoffset")).toBe(String(2 * Math.PI * 54));
  });
});
