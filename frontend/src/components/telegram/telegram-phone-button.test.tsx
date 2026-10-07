import { act, fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import type { TelegramWebApp } from "@/lib/telegram/web-app";
import { TelegramPhoneButton } from "./telegram-phone-button";

let currentWebApp: TelegramWebApp | null = null;
vi.mock("@/components/telegram/telegram-provider", () => ({ useTelegram: () => currentWebApp }));

type Cb = (shared: boolean, res?: { responseUnsafe?: { contact?: { phone_number?: string } } }) => void;
let pending: Cb | null;

function webAppWithContact(): TelegramWebApp {
  return { initData: "signed", requestContact: (cb: Cb) => (pending = cb) } as unknown as TelegramWebApp;
}
const share = (phone?: string) => act(() => pending!(true, { responseUnsafe: { contact: { phone_number: phone } } }));

function renderButton(onPhone = vi.fn()) {
  const view = render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <TelegramPhoneButton onPhone={onPhone} />
    </NextIntlClientProvider>,
  );
  return { onPhone, ...view };
}
const click = () => fireEvent.click(screen.getByRole("button", { name: "Raqamni Telegram'dan olish" }));

beforeEach(() => {
  pending = null;
  currentWebApp = webAppWithContact();
});

describe("TelegramPhoneButton", () => {
  it("renders nothing outside Telegram", () => {
    currentWebApp = null;
    const { container } = renderButton();
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing on a client without requestContact (< 6.9)", () => {
    currentWebApp = { initData: "signed" } as TelegramWebApp;
    const { container } = renderButton();
    expect(container).toBeEmptyDOMElement();
  });

  it("pre-fills the national number from a number with a plus", () => {
    const { onPhone } = renderButton();
    click();
    share("+998901234567");
    expect(onPhone).toHaveBeenCalledWith("901234567");
  });

  it("pre-fills the national number from a number without a plus", () => {
    const { onPhone } = renderButton();
    click();
    share("998901234567");
    expect(onPhone).toHaveBeenCalledWith("901234567");
  });

  it("does nothing when the user declines", () => {
    const { onPhone } = renderButton();
    click();
    act(() => pending!(false));
    expect(onPhone).not.toHaveBeenCalled();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("never drops a foreign number into the field, and says why", () => {
    const { onPhone } = renderButton();
    click();
    share("+79991234567");
    expect(onPhone).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("O'zbekiston");
  });

  it("clears the hint after a good number", () => {
    const { onPhone } = renderButton();
    click();
    share("+1415555");
    click();
    share("+998901234567");
    expect(onPhone).toHaveBeenCalledWith("901234567");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("ignores a callback that fires after unmount", () => {
    const { onPhone, unmount } = renderButton();
    click();
    unmount();
    share("+998901234567");
    expect(onPhone).not.toHaveBeenCalled();
  });
});
