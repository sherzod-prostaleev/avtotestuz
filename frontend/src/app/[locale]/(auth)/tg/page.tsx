import { preconnect } from "react-dom";
import { TelegramEntry } from "@/components/telegram/telegram-entry";
import { TELEGRAM_BOOT_SCRIPT } from "@/lib/telegram/boot-script";
import { configuredBotUsername } from "@/lib/telegram/bot-username";

// The bot username comes from the server environment per request (one image
// serves any deployment), as on /checkout/done.
export const dynamic = "force-dynamic";

export default function TelegramEntryPage() {
  // The SDK is fetched from telegram.org on every Mini App launch; open the
  // connection while the HTML is still arriving.
  preconnect("https://telegram.org");
  return (
    <>
      {/* Before hydration: starts the SDK and sets the launch colour scheme,
          but only inside a real Telegram client (see boot-script.ts). */}
      <script dangerouslySetInnerHTML={{ __html: TELEGRAM_BOOT_SCRIPT }} />
      <TelegramEntry botUsername={configuredBotUsername()} />
    </>
  );
}
