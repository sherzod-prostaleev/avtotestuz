import { render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import { ApiError } from "@/lib/api-client";
import { MustChangePasswordGate } from "./must-change-password-gate";

const replaceMock = vi.fn();
const apiGet = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: replaceMock, push: vi.fn() }),
  usePathname: () => "/uz-Latn/dashboard",
}));

vi.mock("@/lib/api-client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api-client")>();
  return {
    ...actual,
    apiGet: (...args: unknown[]) => apiGet(...args),
  };
});

function renderGate() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <MustChangePasswordGate>
        <p>dashboard-ready</p>
      </MustChangePasswordGate>
    </NextIntlClientProvider>,
  );
}

describe("MustChangePasswordGate", () => {
  beforeEach(() => {
    replaceMock.mockReset();
    apiGet.mockReset();
  });

  it("shows a readable loading status instead of a lone ellipsis while /me is in flight", () => {
    apiGet.mockReturnValue(new Promise(() => {}));
    renderGate();
    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("Sessiya tekshirilmoqda...");
    expect(status.textContent).not.toBe("…");
    expect(screen.queryByText("dashboard-ready")).not.toBeInTheDocument();
  });

  it("renders the app shell after /me says the password is fine", async () => {
    apiGet.mockResolvedValue({ profile: { must_change_password: false } });
    renderGate();
    expect(await screen.findByText("dashboard-ready")).toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it("sends a temporary-password user to change-password", async () => {
    apiGet.mockResolvedValue({ profile: { must_change_password: true } });
    renderGate();
    await waitFor(() => {
      expect(replaceMock).toHaveBeenCalledWith("/uz-Latn/change-password");
    });
    expect(screen.queryByText("dashboard-ready")).not.toBeInTheDocument();
  });

  it("opens the shell when /me times out or fails without 401", async () => {
    apiGet.mockRejectedValue(new DOMException("Aborted", "AbortError"));
    renderGate();
    expect(await screen.findByText("dashboard-ready")).toBeInTheDocument();
  });

  it("sends a 401 to login", async () => {
    apiGet.mockRejectedValue(new ApiError("unauthorized", "unauthorized", 401));
    renderGate();
    await waitFor(() => {
      expect(replaceMock).toHaveBeenCalledWith("/uz-Latn/login");
    });
    expect(screen.queryByText("dashboard-ready")).not.toBeInTheDocument();
  });
});
