import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../messages/uz-Latn.json";
import { WebPushCard } from "../web-push-card";
import * as apiClient from "@/lib/api-client";
import { ApiError } from "@/lib/api-client";

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <WebPushCard />
    </NextIntlClientProvider>
  );
}

describe("WebPushCard", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    Object.defineProperty(window, "PushManager", { value: function PushManager() {}, configurable: true });
    Object.defineProperty(window, "Notification", {
      value: { requestPermission: vi.fn() },
      configurable: true,
    });
    Object.defineProperty(navigator, "serviceWorker", {
      value: {
        getRegistration: vi.fn().mockResolvedValue(undefined),
        register: vi.fn(),
        ready: Promise.resolve({}),
      },
      configurable: true,
    });
  });

  it("shows not-enabled state when configured", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      configured: true,
      subscribed: false,
      subscription_count: 0,
      vapid_public_key: "BPtest",
    });

    renderWithIntl();

    expect(await screen.findByText(/Brauzer bildirishnomalari o'chirilgan/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Yoqish/i })).toBeInTheDocument();
  });

  it("shows unconfigured error when enable hits 503", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      configured: false,
      subscribed: false,
      subscription_count: 0,
    });
    vi.spyOn(apiClient, "apiPost").mockRejectedValue(
      new ApiError("off", "web_push_unconfigured", 503)
    );

    renderWithIntl();
    fireEvent.click(await screen.findByRole("button", { name: /Yoqish/i }));

    await waitFor(() => {
      expect(screen.getByText(/Web push hozircha sozlanmagan/i)).toBeInTheDocument();
    });
  });
});
