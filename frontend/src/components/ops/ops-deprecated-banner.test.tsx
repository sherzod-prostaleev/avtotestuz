import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { OpsDeprecatedBanner } from "./ops-deprecated-banner";

describe("OpsDeprecatedBanner", () => {
  it("links to admin target", () => {
    render(
      <OpsDeprecatedBanner
        locale="uz-Latn"
        href="payments/providers"
        label="Admin providers"
        note="DEPRECATED"
      />,
    );
    const link = screen.getByRole("link", { name: "Admin providers" });
    expect(link.getAttribute("href")).toBe("/uz-Latn/admin/payments/providers");
  });
});
