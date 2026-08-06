import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import messages from "../../../../../../../../messages/uz-Latn.json";
import InstallerPanel from "./installer-panel";

const ORG_ID = "22222222-2222-4222-8222-222222222222";

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function renderPanel() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <InstallerPanel orgId={ORG_ID} />
    </NextIntlClientProvider>,
  );
}

const key = {
  code: "SCHOOL-CODE-1",
  max_uses: 10,
  used_count: 3,
  expires_at: "2027-01-15T00:00:00Z",
};

const rotatedKey = {
  code: "SCHOOL-CODE-2",
  max_uses: 10,
  used_count: 0,
  expires_at: "2027-02-20T00:00:00Z",
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("InstallerPanel", () => {
  it("renders the no-key state and an open button when GET returns data: null", async () => {
    const fetchMock = vi.fn(() => Promise.resolve(json({ data: null })));
    vi.stubGlobal("fetch", fetchMock);

    renderPanel();

    expect(await screen.findByText(messages.AdminB2B.installerNone)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: messages.AdminB2B.installerOpen }),
    ).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      `/api/admin/b2b/orgs/${ORG_ID}/installer`,
      expect.objectContaining({ cache: "no-store" }),
    );
    expect(
      screen.queryByRole("button", { name: messages.AdminB2B.installerRotate }),
    ).not.toBeInTheDocument();
  });

  it("POSTs to open a key and renders the code, used/max and expiry", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (init?.method === "POST" && url === `/api/admin/b2b/orgs/${ORG_ID}/installer`) {
        return Promise.resolve(json({ data: key }));
      }
      return Promise.resolve(json({ data: null }));
    });
    vi.stubGlobal("fetch", fetchMock);
    const user = userEvent.setup();
    renderPanel();

    await screen.findByText(messages.AdminB2B.installerNone);
    await user.click(screen.getByRole("button", { name: messages.AdminB2B.installerOpen }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        `/api/admin/b2b/orgs/${ORG_ID}/installer`,
        expect.objectContaining({ method: "POST" }),
      ),
    );

    expect(await screen.findByText(key.code)).toBeInTheDocument();
    expect(
      screen.getByText(
        messages.AdminB2B.installerUsed
          .replace("{used}", String(key.used_count))
          .replace("{max}", String(key.max_uses)),
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        messages.AdminB2B.installerExpires.replace(
          "{date}",
          new Date(key.expires_at).toLocaleDateString("uz-Latn"),
        ),
      ),
    ).toBeInTheDocument();
  });

  it("asks for confirmation, then POSTs to /rotate and renders the new code", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === `/api/admin/b2b/orgs/${ORG_ID}/installer` && (!init || init.method === undefined)) {
        return Promise.resolve(json({ data: key }));
      }
      if (init?.method === "POST" && url === `/api/admin/b2b/orgs/${ORG_ID}/installer/rotate`) {
        return Promise.resolve(json({ data: rotatedKey }));
      }
      return Promise.resolve(json({ data: key }));
    });
    vi.stubGlobal("fetch", fetchMock);
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(true);
    const user = userEvent.setup();
    renderPanel();

    expect(await screen.findByText(key.code)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: messages.AdminB2B.installerRotate }));

    expect(confirmSpy).toHaveBeenCalledWith(messages.AdminB2B.installerRotateConfirm);
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        `/api/admin/b2b/orgs/${ORG_ID}/installer/rotate`,
        expect.objectContaining({ method: "POST" }),
      ),
    );
    expect(await screen.findByText(rotatedKey.code)).toBeInTheDocument();
    confirmSpy.mockRestore();
  });

  it("does not rotate when confirmation is declined", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === `/api/admin/b2b/orgs/${ORG_ID}/installer` && (!init || init.method === undefined)) {
        return Promise.resolve(json({ data: key }));
      }
      return Promise.resolve(json({ data: key }));
    });
    vi.stubGlobal("fetch", fetchMock);
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValue(false);
    const user = userEvent.setup();
    renderPanel();

    expect(await screen.findByText(key.code)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: messages.AdminB2B.installerRotate }));

    expect(confirmSpy).toHaveBeenCalled();
    expect(
      fetchMock.mock.calls.some(
        ([, init]) => (init as RequestInit | undefined)?.method === "POST",
      ),
    ).toBe(false);
    confirmSpy.mockRestore();
  });

  it("points the download link at installer.exe with the selected locale and updates it when the locale changes", async () => {
    const fetchMock = vi.fn(() => Promise.resolve(json({ data: key })));
    vi.stubGlobal("fetch", fetchMock);
    const user = userEvent.setup();
    renderPanel();

    await screen.findByText(key.code);

    const link = screen.getByRole("link", { name: messages.AdminB2B.installerDownload });
    expect(link).toHaveAttribute(
      "href",
      `/api/admin/b2b/orgs/${ORG_ID}/installer.exe?locale=uz-Latn`,
    );
    expect(link).toHaveAttribute("download");

    await user.selectOptions(screen.getByLabelText(messages.AdminB2B.installerLocale), "ru");

    expect(screen.getByRole("link", { name: messages.AdminB2B.installerDownload })).toHaveAttribute(
      "href",
      `/api/admin/b2b/orgs/${ORG_ID}/installer.exe?locale=ru`,
    );
  });

  it("does not render a download link when no key has been opened yet", async () => {
    const fetchMock = vi.fn(() => Promise.resolve(json({ data: null })));
    vi.stubGlobal("fetch", fetchMock);
    renderPanel();

    await screen.findByText(messages.AdminB2B.installerNone);

    expect(
      screen.queryByRole("link", { name: messages.AdminB2B.installerDownload }),
    ).not.toBeInTheDocument();
  });
});
