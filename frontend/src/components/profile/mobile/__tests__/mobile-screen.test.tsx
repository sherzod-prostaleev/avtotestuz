import { act, fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import { ProfileMobile } from "../profile-mobile";
import * as apiClient from "@/lib/api-client";

const tg = vi.hoisted(() => ({ webApp: null as null | Record<string, unknown> }));
vi.mock("@/components/telegram/telegram-provider", () => ({ useTelegram: () => tg.webApp }));

function fakeTelegramBack() {
  const handlers = new Set<() => void>();
  const BackButton = {
    show: vi.fn(),
    hide: vi.fn(),
    onClick: vi.fn((cb: () => void) => handlers.add(cb)),
    offClick: vi.fn((cb: () => void) => handlers.delete(cb)),
  };
  tg.webApp = { initData: "x", initDataUnsafe: { user: { id: 1 } }, BackButton };
  return { BackButton, press: () => [...handlers].forEach((cb) => cb()) };
}

function renderProfile() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ProfileMobile
        name="Ali"
        region=""
        phone="+998901234567"
        referralCode=""
        isVip={false}
        onNameChange={() => {}}
        onRegionChange={() => {}}
        onSave={() => {}}
        saving={false}
        saved={false}
        errorKey={null}
        onRetry={() => {}}
        onLogout={() => {}}
        loading={false}
      />
    </NextIntlClientProvider>,
  );
}

afterEach(() => {
  tg.webApp = null;
  vi.restoreAllMocks();
});

describe("ProfileMobile sub-panels inside Telegram", () => {
  // Android's back key maps onto Telegram's Back: with it hidden on the
  // profile tab, the key closed the whole Mini App from inside a panel.
  it("shows Telegram's Back in a panel and closes the panel with it", () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({ linked: false });
    const back = fakeTelegramBack();
    renderProfile();
    expect(back.BackButton.show).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: /Telegram/ }));
    expect(back.BackButton.show).toHaveBeenCalledTimes(1);
    act(() => back.press());
    expect(screen.getByText(messages.Profile.connectionsGroup)).toBeInTheDocument();
    expect(back.BackButton.hide).toHaveBeenCalled();
  });
});
