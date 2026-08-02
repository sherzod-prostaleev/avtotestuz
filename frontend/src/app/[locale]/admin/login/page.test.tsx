import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import AdminLoginPage from "./page";
import messages from "../../../../../messages/uz-Latn.json";

const replace = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}));

afterEach(() => {
  vi.unstubAllGlobals();
  replace.mockReset();
});

const t = messages.AdminLogin;

function renderPage() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <AdminLoginPage />
    </NextIntlClientProvider>,
  );
}

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status });
}

async function submitPassword(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(t.email), "root@drivergo.uz");
  await user.type(screen.getByLabelText(t.password), "hunter2hunter2");
  await user.click(screen.getByRole("button", { name: t.submit }));
}

const setupRequired = {
  data: { totp_setup_required: true, expires_in: 900 },
  error: { code: "totp_setup_required", message: "enroll first" },
};

describe("AdminLoginPage TOTP enrollment recovery", () => {
  it("shows the enrollment step with the one-time secret after a totp_setup_required login", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(json(setupRequired, 403))
        .mockResolvedValueOnce(
          json({ data: { secret: "JBSWY3DPEHPK3PXP", otpauth_url: "otpauth://totp/x" } }),
        ),
    );

    renderPage();
    await submitPassword(user);

    expect(await screen.findByText(t.enrollTitle)).toBeInTheDocument();
    expect(await screen.findByText("JBSWY3DPEHPK3PXP")).toBeInTheDocument();
    expect(screen.getByText("otpauth://totp/x")).toBeInTheDocument();
    // The credential itself is never handed to the page.
    expect(screen.queryByText(/enrollment_token/)).not.toBeInTheDocument();
  });

  it("confirms with the code alone and returns to the normal sign-in", async () => {
    const user = userEvent.setup();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(json(setupRequired, 403))
      .mockResolvedValueOnce(json({ data: { secret: "JBSWY3DPEHPK3PXP" } }))
      .mockResolvedValueOnce(json({ data: { totp_enabled: true } }));
    vi.stubGlobal("fetch", fetchMock);

    renderPage();
    await submitPassword(user);
    await screen.findByText("JBSWY3DPEHPK3PXP");

    await user.type(screen.getByPlaceholderText("000000"), "424242");
    await user.click(screen.getByRole("button", { name: t.enrollSubmit }));

    expect(await screen.findByText(t.enrollDone)).toBeInTheDocument();
    expect(screen.getByLabelText(t.password)).toBeInTheDocument();

    const confirmCall = fetchMock.mock.calls[2];
    expect(confirmCall[0]).toBe("/api/admin/security/totp/confirm");
    expect(JSON.parse(confirmCall[1].body as string)).toEqual({ code: "424242" });
    expect(replace).not.toHaveBeenCalled();
  });

  it("explains an expired enrollment token instead of dead-ending", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(json(setupRequired, 403))
        .mockResolvedValueOnce(json({ data: { secret: "JBSWY3DPEHPK3PXP" } }))
        .mockResolvedValueOnce(json({ error: { code: "unauthorized" } }, 401)),
    );

    renderPage();
    await submitPassword(user);
    await screen.findByText("JBSWY3DPEHPK3PXP");
    await user.type(screen.getByPlaceholderText("000000"), "424242");
    await user.click(screen.getByRole("button", { name: t.enrollSubmit }));

    expect(await screen.findByRole("alert")).toHaveTextContent(t.enrollExpired);
    expect(screen.getByRole("button", { name: t.backToPassword })).toBeInTheDocument();
  });

  it("reports a wrong confirmation code without discarding the secret", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(json(setupRequired, 403))
        .mockResolvedValueOnce(json({ data: { secret: "JBSWY3DPEHPK3PXP" } }))
        .mockResolvedValueOnce(json({ error: { code: "invalid_totp" } }, 400)),
    );

    renderPage();
    await submitPassword(user);
    await screen.findByText("JBSWY3DPEHPK3PXP");
    await user.type(screen.getByPlaceholderText("000000"), "000000");
    await user.click(screen.getByRole("button", { name: t.enrollSubmit }));

    expect(await screen.findByRole("alert")).toHaveTextContent(t.errorEnrollCode);
    expect(screen.getByText("JBSWY3DPEHPK3PXP")).toBeInTheDocument();
  });

  it("offers a retry when the enroll call itself fails", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(json(setupRequired, 403))
        .mockResolvedValueOnce(json({ error: { code: "unauthorized" } }, 401))
        .mockResolvedValueOnce(json({ data: { secret: "RETRIEDSECRET" } })),
    );

    renderPage();
    await submitPassword(user);

    expect(await screen.findByRole("alert")).toHaveTextContent(t.enrollExpired);
    await user.click(screen.getByRole("button", { name: t.enrollRetry }));
    expect(await screen.findByText("RETRIEDSECRET")).toBeInTheDocument();
  });
});

describe("AdminLoginPage ordinary sign-in", () => {
  it("still goes password -> totp challenge -> admin", async () => {
    const user = userEvent.setup();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(json({ data: { requires_totp: true, challenge_token: "ch.1" } }))
      .mockResolvedValueOnce(json({ data: { ok: true } }));
    vi.stubGlobal("fetch", fetchMock);

    renderPage();
    await submitPassword(user);

    await user.type(await screen.findByPlaceholderText("000000"), "123456");
    await user.click(screen.getByRole("button", { name: t.totpSubmit }));

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/uz-Latn/admin"));
    expect(JSON.parse(fetchMock.mock.calls[1][1].body as string)).toEqual({
      challenge_token: "ch.1",
      code: "123456",
    });
  });

  it("shows the credential error and stays on the password step", async () => {
    const user = userEvent.setup();
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(json({ error: { code: "invalid_credentials" } }, 401)),
    );

    renderPage();
    await submitPassword(user);

    expect(await screen.findByRole("alert")).toHaveTextContent(t.errorCreds);
    expect(screen.getByLabelText(t.password)).toBeInTheDocument();
  });
});
