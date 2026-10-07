import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../../../messages/uz-Latn.json";
import CheckoutDonePage from "./page";
import CheckoutDoneBotPage from "./[bot]/page";
import { PROTECTED_SEGMENTS, isProtectedPath, matchesAny } from "@/lib/protected-segments";

async function renderBot(bot: string) {
  const ui = await CheckoutDoneBotPage({ params: Promise.resolve({ bot }) });
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      {ui}
    </NextIntlClientProvider>,
  );
}

function renderBare() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      {CheckoutDonePage()}
    </NextIntlClientProvider>,
  );
}

describe("/checkout/done (Mini App payment return)", () => {
  beforeEach(() => vi.stubEnv("TELEGRAM_BOT_USERNAME", "AvtoTest_bot"));
  afterEach(() => vi.unstubAllEnvs());

  it("confirms the payment and links back to the bot", async () => {
    await renderBot("AvtoTest_bot");
    expect(screen.getByRole("heading", { name: messages.Premium.checkoutDoneTitle })).toBeInTheDocument();
    const back = screen.getByRole("link", { name: messages.Premium.checkoutDoneBackToBot });
    expect(back).toHaveAttribute("href", "https://t.me/AvtoTest_bot");
  });

  it("matches the configured bot case-insensitively and links to the configured name", async () => {
    await renderBot("avtotest_BOT");
    const back = screen.getByRole("link", { name: messages.Premium.checkoutDoneBackToBot });
    expect(back).toHaveAttribute("href", "https://t.me/AvtoTest_bot");
  });

  // The path is attacker-editable: our page must never vouch for someone
  // else's bot ("payment done, continue in @evil_bot").
  it("shows text only for a bot that is not ours", async () => {
    await renderBot("Evil_payment_bot");
    expect(screen.getByText(messages.Premium.checkoutDoneNoBot)).toBeInTheDocument();
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("shows text only when no bot is configured", async () => {
    vi.stubEnv("TELEGRAM_BOT_USERNAME", "");
    await renderBot("AvtoTest_bot");
    expect(screen.getByText(messages.Premium.checkoutDoneNoBot)).toBeInTheDocument();
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("shows text only without a bot segment", () => {
    renderBare();
    expect(screen.getByText(messages.Premium.checkoutDoneNoBot)).toBeInTheDocument();
    expect(screen.queryByRole("link")).toBeNull();
  });

  it.each([
    ["too short", "abc"],
    ["a different host", "evil.com/x"],
    ["a path trick", "bot_name/../../x"],
    ["an encoded path trick", "bot%2F..%2Fx"],
    ["an @ prefix", "@avtotest_bot"],
    ["too long", "a".repeat(33)],
  ])("shows text only, never a 404, when the bot is %s", async (_name, bot) => {
    await renderBot(bot);
    expect(screen.getByRole("heading", { name: messages.Premium.checkoutDoneTitle })).toBeInTheDocument();
    expect(screen.getByText(messages.Premium.checkoutDoneNoBot)).toBeInTheDocument();
    expect(screen.queryByRole("link")).toBeNull();
  });

  // The external browser that Payme/Click return to has no session: the page
  // must not sit behind the proxy's login redirect like the rest of /checkout.
  it("is public while the rest of /checkout stays protected", () => {
    expect(matchesAny("/checkout/done", PROTECTED_SEGMENTS)).toBe(true);
    expect(isProtectedPath("/checkout/done")).toBe(false);
    expect(isProtectedPath("/checkout/pending")).toBe(true);
    expect(isProtectedPath("/checkout/done/avtotest_bot")).toBe(false);
    expect(isProtectedPath("/checkout/done/a/b")).toBe(true);
    expect(isProtectedPath("/checkout/done/")).toBe(true);
    expect(isProtectedPath("/checkout/pending/x")).toBe(true);
    expect(isProtectedPath("/checkout")).toBe(true);
  });
});
