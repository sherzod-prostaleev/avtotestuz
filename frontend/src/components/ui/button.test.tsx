import { render } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { Button } from "./button";

describe("Button", () => {
  it("renders children correctly", () => {
    const { getByRole } = render(<Button>Davom etish</Button>);
    expect(getByRole("button", { name: "Davom etish" })).toBeInTheDocument();
  });

  it("defaults to standard 3d rounded variant when no variant is given", () => {
    const { getByRole } = render(<Button>Davom etish</Button>);
    const button = getByRole("button", { name: "Davom etish" });
    expect(button.className).toContain("rounded-xl");
    expect(button.className).not.toContain("rounded-full");
  });

  it("renders as a non-interactive span for Link nesting", () => {
    const { container, queryByRole } = render(
      <Button as="span" variant="game">
        Katalogni ko&apos;rish
      </Button>
    );
    expect(queryByRole("button")).toBeNull();
    expect(container.querySelector("span")?.className).toContain("bg-accent");
  });
});
