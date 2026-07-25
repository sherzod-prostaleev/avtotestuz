import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { OpsNav } from "./ops-nav";

vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

describe("OpsNav", () => {
  it("marks the active ops section", () => {
    render(
      <OpsNav locale="uz-Latn" active="health" healthLabel="Holat" providersLabel="Provayderlar" />
    );
    expect(screen.getByRole("link", { name: "Holat" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Provayderlar" })).not.toHaveAttribute("aria-current");
    expect(screen.getByRole("link", { name: "Holat" })).toHaveAttribute("href", "/uz-Latn/ops/health");
  });
});
