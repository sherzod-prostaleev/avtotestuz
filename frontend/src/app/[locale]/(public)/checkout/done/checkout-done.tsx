import type { Metadata } from "next";
import { useTranslations } from "next-intl";
import { Clock, Send } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { BOT_USERNAME, configuredBotUsername } from "@/lib/telegram/bot-username";

// A payment return screen: nothing to rank, and indexing it would only send
// searchers to a "thanks for paying" page.
export const doneMetadata: Metadata = { robots: { index: false, follow: false } };

/**
 * The bot to link back to: only OUR bot. The path segment is attacker-editable
 * (anyone can send a payer to /checkout/done/evil_bot), so it must equal the
 * configured TELEGRAM_BOT_USERNAME (Telegram usernames are case-insensitive);
 * the link then uses the configured spelling. Read at request time so one
 * image serves any deployment. Missing or different → text only.
 */
export function botFrom(value: string | string[] | undefined): string | null {
  if (typeof value !== "string" || !BOT_USERNAME.test(value)) return null;
  const configured = configuredBotUsername();
  if (!configured) return null;
  return configured.toLowerCase() === value.toLowerCase() ? configured : null;
}

/**
 * Where Payme/Click return a payer who started in the Telegram Mini App. They
 * land in an external browser that has no session, so this page is public
 * (proxy isProtectedPath) and does not poll anything: the Mini App's own
 * pending screen confirms the entitlement. It only says the payment is being
 * confirmed and points back to the bot.
 */
export function CheckoutDone({ bot }: { bot: string | null }) {
  const t = useTranslations("Premium");
  return (
    <main className="mx-auto flex min-h-[70vh] max-w-md flex-col items-center justify-center p-4 text-center">
      <Card className="flex w-full flex-col items-center p-6 sm:p-8">
        <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-full border-2 border-border bg-muted text-muted-foreground">
          <Clock aria-hidden="true" className="h-8 w-8" />
        </div>
        <h1 className="font-display text-2xl font-extrabold tracking-tight text-foreground sm:text-3xl">
          {t("checkoutDoneTitle")}
        </h1>
        <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{t("checkoutDoneBody")}</p>
        {bot ? (
          <a href={`https://t.me/${bot}`} className="mt-6 w-full">
            <Button as="span" variant="game" size="lg" className="w-full">
              <Send aria-hidden="true" className="mr-2 h-4 w-4" />
              {t("checkoutDoneBackToBot")}
            </Button>
          </a>
        ) : (
          <p className="mt-4 text-sm font-semibold text-foreground">{t("checkoutDoneNoBot")}</p>
        )}
      </Card>
    </main>
  );
}
