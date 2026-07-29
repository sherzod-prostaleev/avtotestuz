import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { AdminBreadcrumbs } from "./admin-breadcrumbs";
import messages from "../../../messages/uz-Latn.json";

/**
 * Since the shell rewrite moved the operator chip into the sidebar, the trail
 * is the only thing left in the header that says where you are — so its
 * root-suppression and its leaf marking are load-bearing, not decoration.
 */
function renderTrail(pathname: string) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <AdminBreadcrumbs locale="uz-Latn" pathname={pathname} />
    </NextIntlClientProvider>,
  );
}

describe("AdminBreadcrumbs", () => {
  it("shows only the home crumb at the admin root", () => {
    renderTrail("/uz-Latn/admin");
    const items = screen.getAllByRole("listitem");
    expect(items).toHaveLength(1);
    expect(items[0]).toHaveTextContent("Boshqaruv markazi");
  });

  it("marks the root crumb as the current page there", () => {
    renderTrail("/uz-Latn/admin");
    expect(screen.getByRole("link", { name: /Boshqaruv markazi/ })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("builds group then leaf for a nested route", () => {
    renderTrail("/uz-Latn/admin/payments/transactions");
    expect(screen.getByText("To‘lovlar")).toBeInTheDocument();
    const leaf = screen.getByText("Tranzaksiyalar");
    expect(leaf).toHaveAttribute("aria-current", "page");
  });

  it("keeps the leaf on a detail route under that leaf", () => {
    renderTrail("/uz-Latn/admin/payments/transactions/abc-123");
    expect(screen.getByText("Tranzaksiyalar")).toHaveAttribute("aria-current", "page");
  });

  it("falls back to the home crumb alone on an unknown route", () => {
    renderTrail("/uz-Latn/admin/nowhere/at/all");
    expect(screen.getAllByRole("listitem")).toHaveLength(1);
  });

  it("keeps the separators out of the list's content model", () => {
    const { container } = renderTrail("/uz-Latn/admin/payments/transactions");
    // <ol> may only contain <li>; a bare <svg> separator is invalid markup.
    for (const child of Array.from(container.querySelectorAll("ol > *"))) {
      expect(child.tagName.toLowerCase()).toBe("li");
    }
  });
});
