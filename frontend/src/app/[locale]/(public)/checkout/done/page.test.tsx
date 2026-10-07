import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it } from "vitest";
import messages from "../../../../../../messages/uz-Latn.json";
import CheckoutDonePage from "./page";
import { PROTECTED_SEGMENTS, isProtectedPath, matchesAny } from "@/lib/protected-segments";

async function renderPage(search: Record<string, string | string[] | undefined>) {
  const ui = await CheckoutDonePage({ searchParams: Promise.resolve(search) });
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      {ui}
    </NextIntlClientProvider>,
  );
}

describe("/checkout/done (Mini App payment return)", () => {
  it("confirms the payment and links back to the bot", async () => {
    await renderPage({ bot: "AvtoTest_bot" });
    expect(screen.getByRole("heading", { name: messages.Premium.checkoutDoneTitle })).toBeInTheDocument();
    const back = screen.getByRole("link", { name: messages.Premium.checkoutDoneBackToBot });
    expect(back).toHaveAttribute("href", "https://t.me/AvtoTest_bot");
  });

  it.each([
    ["missing", undefined],
    ["too short", "abc"],
    ["a different host", "evil.com/x"],
    ["a path trick", "bot_name/../../x"],
    ["an @ prefix", "@avtotest_bot"],
    ["too long", "a".repeat(33)],
    ["repeated", ["avtotest_bot", "other_bot"]],
  ])("shows text only when the bot is %s", async (_name, bot) => {
    await renderPage({ bot });
    expect(screen.getByText(messages.Premium.checkoutDoneNoBot)).toBeInTheDocument();
    expect(screen.queryByRole("link")).toBeNull();
  });

  // The external browser that Payme/Click return to has no session: the page
  // must not sit behind the proxy's login redirect like the rest of /checkout.
  it("is public while the rest of /checkout stays protected", () => {
    expect(matchesAny("/checkout/done", PROTECTED_SEGMENTS)).toBe(true);
    expect(isProtectedPath("/checkout/done")).toBe(false);
    expect(isProtectedPath("/checkout/pending")).toBe(true);
    expect(isProtectedPath("/checkout/done/x")).toBe(true);
    expect(isProtectedPath("/checkout")).toBe(true);
  });
});
