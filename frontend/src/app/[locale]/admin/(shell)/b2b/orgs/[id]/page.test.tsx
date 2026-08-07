import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { AdminMeProvider } from "@/components/admin/admin-me-context";
import messages from "../../../../../../../../messages/uz-Latn.json";
import AdminB2BOrgDetailPage from "./page";

const replace = vi.fn();
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "22222222-2222-4222-8222-222222222222" }),
  useRouter: () => ({ replace }),
}));

const detail = {
  org: {
    id: "22222222-2222-4222-8222-222222222222",
    name: "Chilonzor Avtomaktab",
    status: "active",
    active_seats: 30,
  },
  licenses: [],
  stations: [],
  seats_used: 0,
};

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function renderPage() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <AdminMeProvider
        me={{
          id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
          email: "ops@example.uz",
          display_name: "Ops",
          roles: ["superadmin"],
          permissions: ["users.read", "b2b.orgs.hard_delete"],
        }}
      >
        <AdminB2BOrgDetailPage />
      </AdminMeProvider>
    </NextIntlClientProvider>,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  replace.mockReset();
});

describe("Admin B2B organization detail", () => {
  it("explains the station model and requires the exact org name before deletion", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "DELETE") return Promise.resolve(json({ data: { deleted: true } }));
      if (url.endsWith("/stats")) {
        return Promise.resolve(
          json({
            data: {
              active_seats: 30,
              seats_used: 0,
            },
          }),
        );
      }
      return Promise.resolve(json({ data: detail }));
    });
    vi.stubGlobal("fetch", fetchMock);
    const user = userEvent.setup();
    renderPage();

    expect(await screen.findByText(messages.AdminB2B.multiPcTitle)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: messages.AdminB2B.deleteButton }));
    const confirmButton = screen.getByRole("button", {
      name: messages.AdminB2B.deleteButton,
    });
    expect(confirmButton).toBeDisabled();
    await user.type(screen.getByRole("textbox", { name: /avtomaktab nomini/i }), detail.org.name);
    expect(confirmButton).toBeEnabled();
    await user.click(confirmButton);

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        `/api/admin/b2b/orgs/${detail.org.id}`,
        expect.objectContaining({
          method: "DELETE",
          body: JSON.stringify({ confirm: detail.org.name }),
        }),
      ),
    );
    expect(replace).toHaveBeenCalledWith("/uz-Latn/admin/b2b/orgs");
  });
});
