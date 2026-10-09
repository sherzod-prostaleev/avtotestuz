import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, describe, expect, it, vi } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import ruMessages from "../../../messages/ru.json";
import cyrlMessages from "../../../messages/uz-Cyrl.json";
import { rememberSignedInAs } from "@/lib/signed-in-notice";
import { SignedInNotice } from "./signed-in-notice";

afterEach(() => {
  window.sessionStorage.clear();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

function renderNotice(locale = "uz-Latn", m: object = messages) {
  return render(
    <NextIntlClientProvider locale={locale} messages={m}>
      <SignedInNotice />
    </NextIntlClientProvider>
  );
}

describe("SignedInNotice", () => {
  it("renders nothing without a fresh Telegram login", () => {
    const { container } = renderNotice();
    expect(container).toBeEmptyDOMElement();
  });

  it("says once which account was signed in, in every locale", async () => {
    for (const [locale, m, text] of [
      ["uz-Latn", messages, "+998 90 ••• •• 67 raqami bilan kirdingiz"],
      ["uz-Cyrl", cyrlMessages, "+998 90 ••• •• 67 рақами билан кирдингиз"],
      ["ru", ruMessages, "Вы вошли с номером +998 90 ••• •• 67"],
    ] as const) {
      rememberSignedInAs("+998 90 ••• •• 67");
      const { unmount } = renderNotice(locale, m);
      // toHaveTextContent normalises the no-break spaces inside the number.
      expect(await screen.findByRole("status")).toHaveTextContent(text);
      unmount();
      // Shown once: the next screen has nothing to show.
      const again = renderNotice(locale, m);
      expect(again.container).toBeEmptyDOMElement();
      again.unmount();
    }
  });

  it("can be dismissed, and goes away by itself", async () => {
    rememberSignedInAs("+998 90 ••• •• 67");
    const first = renderNotice();
    fireEvent.click(await screen.findByRole("button", { name: "Yopish" }));
    expect(screen.queryByRole("status")).toBeNull();
    first.unmount();

    vi.useFakeTimers();
    rememberSignedInAs("+998 90 ••• •• 67");
    renderNotice();
    expect(screen.getByRole("status")).toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(20_000);
    });
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("lets someone who sees a number that is not theirs sign out", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const assign = vi.fn();
    vi.stubGlobal("location", { ...window.location, assign });
    rememberSignedInAs("+998 90 ••• •• 67");
    renderNotice();
    fireEvent.click(await screen.findByRole("button", { name: "Bu sizning raqamingiz emasmi? Chiqish" }));
    await waitFor(() => expect(assign).toHaveBeenCalledWith("/uz-Latn/login"));
    expect(fetchMock).toHaveBeenCalledWith("/api/auth/logout", { method: "POST" });
  });
});
