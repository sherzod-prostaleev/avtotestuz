import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { AdminMeProvider } from "@/components/admin/admin-me-context";
import messages from "../../../../../../../messages/uz-Latn.json";
import AdminUserDetailPage from "./page";

const replace = vi.fn();
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "11111111-1111-4111-8111-111111111111" }),
  useRouter: () => ({ replace }),
}));

const learner = {
  id: "11111111-1111-4111-8111-111111111111",
  phone: "+998901112233",
  phone_masked: "+998***2233",
  name: "Ali Valiyev",
  region: "",
  district: "",
  locale_pref: "uz-Latn",
  theme_pref: "light",
  role: "user",
  status: "active",
  vip_active: false,
  has_password: true,
  streak: 0,
  bypass_variant_progress: false,
  created_at: "2026-08-01T00:00:00Z",
  entitlements: [],
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
          permissions: ["users.read", "users.hard_delete"],
        }}
      >
        <AdminUserDetailPage />
      </AdminMeProvider>
    </NextIntlClientProvider>,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  replace.mockReset();
});

describe("Admin user hard delete", () => {
  it.each([false, true])("copies the full referral link, clipboard failure=%s", async (fails) => {
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith("/sessions")) return Promise.resolve(json({ data: [] }));
      if (url.endsWith("/referral") || url.endsWith("/ledger")) return Promise.resolve(json({}, 403));
      return Promise.resolve(json({ data: { ...learner, referral_code: "REF-C62LC2", referral_invite_url: "https://drivergo.uz/r/REF-C62LC2" } }));
    }));
    const user = userEvent.setup();
    const write = vi.spyOn(navigator.clipboard, "writeText");
    if (fails) write.mockRejectedValueOnce(new Error("denied"));
    renderPage();
    await user.click(await screen.findByRole("button", { name: messages.AdminUsers.copyReferralLink }));
    expect(write).toHaveBeenCalledWith("https://drivergo.uz/r/REF-C62LC2");
    expect(await screen.findByText(fails ? messages.AdminUsers.referralCopyError : messages.AdminUsers.referralLinkCopied)).toBeInTheDocument();
    write.mockRestore();
  });

  it("shows the protected danger action and sends the typed phone confirmation", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "DELETE") return Promise.resolve(json({ data: { deleted: true } }));
      if (url.endsWith("/sessions")) return Promise.resolve(json({ data: [] }));
      if (url.endsWith("/referral") || url.endsWith("/referral/ledger")) {
        return Promise.resolve(json({ error: { code: "forbidden" } }, 403));
      }
      return Promise.resolve(json({ data: learner }));
    });
    vi.stubGlobal("fetch", fetchMock);
    const user = userEvent.setup();
    renderPage();

    expect(await screen.findByRole("heading", { name: "Ali Valiyev" })).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: messages.AdminUsers.actionsTab }));
    await user.click(screen.getByRole("button", { name: messages.AdminUsers.deleteButton }));

    const confirmButton = screen.getByRole("button", {
      name: messages.AdminUsers.deleteButton,
    });
    expect(confirmButton).toBeDisabled();
    await user.type(screen.getByRole("textbox", { name: /telefon raqamini/i }), learner.phone);
    expect(confirmButton).toBeEnabled();
    await user.click(confirmButton);

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        `/api/admin/users/${learner.id}`,
        expect.objectContaining({
          method: "DELETE",
          body: JSON.stringify({ confirm: learner.phone }),
        }),
      ),
    );
    expect(replace).toHaveBeenCalledWith("/uz-Latn/admin/users");
  });
});
