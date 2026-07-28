import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { AdminMobileBar } from "./admin-mobile-bar";
import { AdminMeProvider } from "./admin-me-context";
import messages from "../../../messages/uz-Latn.json";

// Four of the five thumb-zone destinations are permission-gated routes
// (payments, users, monitoring, support). The bar must answer the same way
// the sidebar does — it is the phone's only nav, so an item here is a promise.
const ALL_PERMISSIONS = [
  "payments.read",
  "users.read",
  "monitoring.read",
  "support.inbox",
];

function renderBar(activePath: string, permissions: string[] = ALL_PERMISSIONS) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <AdminMeProvider
        me={{
          id: "1",
          email: "admin@example.com",
          display_name: "Admin",
          roles: ["admin"],
          permissions,
        }}
      >
        <AdminMobileBar locale="uz-Latn" activePath={activePath} />
      </AdminMeProvider>
    </NextIntlClientProvider>,
  );
}

describe("AdminMobileBar", () => {
  it("renders the five critical destinations", () => {
    renderBar("/uz-Latn/admin");
    const nav = screen.getByRole("navigation", { name: "Asosiy bo‘limlar" });
    expect(within(nav).getAllByRole("link")).toHaveLength(5);
  });

  it("marks the current destination", () => {
    renderBar("/uz-Latn/admin/payments/manual");
    expect(screen.getByRole("link", { name: /Manual Humo/ })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("does not mark overview as current on a nested route", () => {
    renderBar("/uz-Latn/admin/users");
    expect(screen.getByRole("link", { name: /Umumiy/ })).not.toHaveAttribute("aria-current");
  });

  it("hides destinations the admin has no permission for", () => {
    renderBar("/uz-Latn/admin", ["support.inbox"]);
    const nav = screen.getByRole("navigation", { name: "Asosiy bo‘limlar" });
    // Overview is ungated, support.inbox is held -> 2 links, nothing else.
    expect(within(nav).getAllByRole("link")).toHaveLength(2);
    expect(screen.queryByRole("link", { name: /Manual Humo/ })).not.toBeInTheDocument();
  });

  it("fails closed when the admin has no permissions at all", () => {
    renderBar("/uz-Latn/admin", []);
    const nav = screen.getByRole("navigation", { name: "Asosiy bo‘limlar" });
    expect(within(nav).getAllByRole("link")).toHaveLength(1);
  });
});
