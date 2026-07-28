import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { AdminSidebar } from "./admin-sidebar";
import { activeGroupTitleKey, adminNav } from "./admin-nav-config";
import { AdminMeProvider } from "./admin-me-context";
// Use the real catalogue: the sidebar renders every group, so a hand-written
// subset would make next-intl warn about missing keys and mask real gaps.
import messages from "../../../messages/uz-Latn.json";

// AdminSidebar filters every item through the same permission gate it uses in
// production (routePermissionKey -> ADMIN_ROUTE_PERMISSIONS -> hasPermission),
// and that gate fails closed when there is no admin context (see
// permission-gate.test.tsx for the established pattern). In production the
// sidebar is only ever mounted inside AdminMeProvider (the shell layout
// returns null until `me` loads), so an admin with full permissions here
// reflects reality and lets these tests exercise presentation, not the
// security-relevant permission logic itself — which this task must not touch.
const ALL_PERMISSIONS = [
  "monitoring.read",
  "analytics.read",
  "investors.read",
  "users.read",
  "content.questions.read",
  "payments.read",
  "referral.read",
  "cms.read",
  "settings.flags",
  "settings.config",
  "security.audit.read",
  "security.rbac",
  "support.inbox",
  "support.broadcast",
];

function renderSidebar(activePath: string, permissions: string[] = ALL_PERMISSIONS) {
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
        <AdminSidebar locale="uz-Latn" activePath={activePath} />
      </AdminMeProvider>
    </NextIntlClientProvider>,
  );
}

describe("adminNav config", () => {
  it("gives every group an icon", () => {
    for (const group of adminNav("uz-Latn")) {
      expect(group.icon, `${group.titleKey} has no icon`).toBeTruthy();
    }
  });

  it("resolves the active group from a nested pathname", () => {
    const groups = adminNav("uz-Latn");
    expect(activeGroupTitleKey(groups, "/uz-Latn/admin/payments/transactions/abc")).toBe(
      "groupPayments",
    );
    expect(activeGroupTitleKey(groups, "/uz-Latn/admin/nowhere")).toBeNull();
  });
});

describe("AdminSidebar", () => {
  it("opens only the active group by default", () => {
    renderSidebar("/uz-Latn/admin/users");
    expect(screen.getByRole("link", { name: /Katalog/ })).toBeInTheDocument();
    // A link from a non-active group is not rendered while collapsed.
    expect(screen.queryByRole("link", { name: /Tranzaksiyalar/ })).not.toBeInTheDocument();
  });

  it("expands a collapsed group when its header is clicked", async () => {
    const user = userEvent.setup();
    renderSidebar("/uz-Latn/admin/users");
    await user.click(screen.getByRole("button", { name: /To‘lovlar/ }));
    expect(screen.getByRole("link", { name: /Tranzaksiyalar/ })).toBeInTheDocument();
  });

  it("marks the active link with aria-current", () => {
    renderSidebar("/uz-Latn/admin/users");
    expect(screen.getByRole("link", { name: /Katalog/ })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("uses design tokens, not hard-coded colors", () => {
    const { container } = renderSidebar("/uz-Latn/admin/users");
    expect(container.innerHTML).not.toMatch(/hsl\(220_28%_7%\)/);
  });
});

// The sidebar is the operator's map of the panel: a group it renders is a
// capability the operator believes they hold. These tests pin the deny path so
// a future refactor of the filter cannot silently widen it.
describe("AdminSidebar permission gate", () => {
  it("hides groups whose permission the admin lacks", () => {
    renderSidebar("/uz-Latn/admin/users", ["users.read"]);
    // users.read is held -> its group is there, and so is B2B (same permission).
    expect(screen.getByRole("button", { name: /Foydalanuvchilar/ })).toBeInTheDocument();
    // Everything gated behind another permission is gone entirely — header included.
    expect(screen.queryByRole("button", { name: /To‘lovlar/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Xavfsizlik/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Kontent/ })).not.toBeInTheDocument();
  });

  it("honours any-of permissions", () => {
    // payments maps to ["payments.read", "referral.read"] — either one opens it.
    renderSidebar("/uz-Latn/admin/users", ["referral.read"]);
    expect(screen.getByRole("button", { name: /To‘lovlar/ })).toBeInTheDocument();
  });

  it("fails closed when the admin has no permissions at all", () => {
    renderSidebar("/uz-Latn/admin", []);
    for (const label of [
      /Foydalanuvchilar/,
      /To‘lovlar/,
      /Xavfsizlik/,
      /Kontent/,
      /Nazorat/,
      /Analitika/,
      /Investorlar/,
      /Sozlamalar/,
      /Qo‘llab-quvvatlash/,
    ]) {
      expect(screen.queryByRole("button", { name: label }), `${label} leaked`).not.toBeInTheDocument();
    }
  });
});
