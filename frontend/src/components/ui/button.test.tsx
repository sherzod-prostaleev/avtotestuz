import { render } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { Button } from "./button";

describe("Button", () => {
  it("applies pill shape and 3D press-shadow classes for the game variant", () => {
    const { getByRole } = render(<Button variant="game">Boshlash</Button>);
    const button = getByRole("button", { name: "Boshlash" });
    expect(button.className).toContain("rounded-full");
    expect(button.className).toContain("active:translate-y-1");
  });

  it("defaults to the standard rounded-md variant when no variant is given", () => {
    const { getByRole } = render(<Button>Davom etish</Button>);
    const button = getByRole("button", { name: "Davom etish" });
    expect(button.className).toContain("rounded-md");
    expect(button.className).not.toContain("rounded-full");
  });
});
