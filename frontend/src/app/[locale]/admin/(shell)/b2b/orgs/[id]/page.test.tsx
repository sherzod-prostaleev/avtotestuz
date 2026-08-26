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

  // The row of a revoked PC must offer the way back, and only that row. This
  // is the control that was missing on 2026-08-26, when 37 revoked classroom
  // PCs could only be restored with a hand-written UPDATE against production.
  it("offers a revoked PC the way back, and an active one only the way out", async () => {
    const withStations = {
      ...detail,
      stations: [
        { id: "33333333-3333-4333-8333-333333333333", label: "PC-LIVE", status: "active" },
        { id: "44444444-4444-4444-8444-444444444444", label: "PC-DOWN", status: "revoked" },
      ],
      seats_used: 1,
    };
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/stats")) {
        return Promise.resolve(json({ data: { active_seats: 30, seats_used: 1 } }));
      }
      if (url.endsWith("/reactivate")) {
        return Promise.resolve(json({ data: { reactivated: true } }));
      }
      return Promise.resolve(json({ data: withStations }));
    });
    vi.stubGlobal("fetch", fetchMock);
    const user = userEvent.setup();
    renderPage();

    expect(await screen.findByText(/PC-DOWN/)).toBeInTheDocument();

    // One of each: the live PC can be disconnected, the revoked one restored.
    const restore = screen.getAllByRole("button", { name: messages.AdminB2B.reactivateStation });
    expect(restore).toHaveLength(1);
    expect(
      screen.getAllByRole("button", { name: messages.AdminB2B.revokeStation }),
    ).toHaveLength(1);

    await user.click(restore[0]);
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        `/api/admin/b2b/orgs/${detail.org.id}/stations/44444444-4444-4444-8444-444444444444/reactivate`,
        expect.objectContaining({ method: "POST" }),
      ),
    );
  });

  // A full licence is the refusal an operator will actually hit, and "could
  // not reactivate" would send them hunting for a fault that isn't there.
  it("says a full licence is why the PC did not come back", async () => {
    const withStations = {
      ...detail,
      stations: [{ id: "44444444-4444-4444-8444-444444444444", label: "PC-DOWN", status: "revoked" }],
    };
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/stats")) {
        return Promise.resolve(json({ data: { active_seats: 30, seats_used: 30 } }));
      }
      if (url.endsWith("/reactivate")) {
        return Promise.resolve(
          json({ error: { code: "seats_exhausted", message: "full" } }, 409),
        );
      }
      return Promise.resolve(json({ data: withStations }));
    });
    vi.stubGlobal("fetch", fetchMock);
    const user = userEvent.setup();
    renderPage();

    await user.click(
      await screen.findByRole("button", { name: messages.AdminB2B.reactivateStation }),
    );
    expect(await screen.findByText(messages.AdminB2B.errorReactivateSeats)).toBeInTheDocument();
  });
});
