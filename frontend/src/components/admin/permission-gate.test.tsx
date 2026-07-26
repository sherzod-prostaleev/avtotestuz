import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { PermissionGate } from "./permission-gate";
import { AdminMeProvider } from "./admin-me-context";
import { hasPermission } from "@/lib/admin-permissions";

const messages = {
  AdminShell: {
    permissionDeniedTitle: "Ruxsat yo‘q",
    permissionDeniedBody: "Bu bo‘lim uchun ruxsat berilmagan.",
  },
};

function wrap(ui: React.ReactNode, permissions: string[]) {
  return (
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <AdminMeProvider
        me={{
          id: "1",
          email: "a@b.uz",
          display_name: "A",
          roles: ["admin"],
          permissions,
        }}
      >
        {ui}
      </AdminMeProvider>
    </NextIntlClientProvider>
  );
}

describe("hasPermission", () => {
  it("matches any-of", () => {
    expect(hasPermission(["users.read"], ["users.write", "users.read"])).toBe(true);
    expect(hasPermission(["users.read"], "users.write")).toBe(false);
  });
});

describe("PermissionGate", () => {
  it("renders children when permitted", () => {
    render(wrap(<PermissionGate permission="users.read">ok</PermissionGate>, ["users.read"]));
    expect(screen.getByText("ok")).toBeInTheDocument();
  });

  it("shows deny state when missing", () => {
    render(wrap(<PermissionGate permission="users.write">secret</PermissionGate>, ["users.read"]));
    expect(screen.queryByText("secret")).not.toBeInTheDocument();
    expect(screen.getByText("Ruxsat yo‘q")).toBeInTheDocument();
  });

  it("hides in hide mode", () => {
    const { container } = render(
      wrap(
        <PermissionGate permission="users.write" mode="hide">
          secret
        </PermissionGate>,
        ["users.read"],
      ),
    );
    expect(container.textContent).toBe("");
  });
});
