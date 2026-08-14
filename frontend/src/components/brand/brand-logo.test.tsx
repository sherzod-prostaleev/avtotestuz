import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { BrandLogo } from "./brand-logo";

describe("BrandLogo", () => {
  it("renders the static 48px WebP chrome mark", () => {
    render(<BrandLogo alt="Driver Go" size={40} />);
    const img = screen.getByAltText("Driver Go");
    expect(img.getAttribute("src")).toBe("/logo-48.webp");
    expect(img).toHaveAttribute("width", "40");
    expect(img).toHaveAttribute("height", "40");
  });

  it("defaults to decorative empty alt", () => {
    const { container } = render(<BrandLogo />);
    const img = container.querySelector("img");
    expect(img).toHaveAttribute("alt", "");
  });
});
