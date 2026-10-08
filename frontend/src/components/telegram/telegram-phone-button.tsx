"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Smartphone } from "lucide-react";
import { useTelegram } from "@/components/telegram/telegram-provider";
import { nationalPhoneFromShared } from "@/lib/phone-format";

/**
 * Telegram's own "share your number" sheet. Pre-fills the field — the number
 * still goes through the same validation and password check as typing it
 * (spec D3) — and hands over Telegram's signed response, which the server
 * needs to link this Telegram account (it checks the signature itself; the
 * client's copy is never trusted).
 */
export function TelegramPhoneButton({
  onPhone,
}: {
  onPhone: (national: string, signedContact: string | null) => void;
}) {
  const t = useTranslations("TelegramApp");
  const webApp = useTelegram();
  const [foreign, setForeign] = useState(false);
  // The sheet is native UI: its callback can land after we are gone.
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  // Outside Telegram, and clients older than 6.9 (no requestContact), get no button.
  if (!webApp || typeof webApp.requestContact !== "function") return null;

  function share() {
    setForeign(false);
    webApp!.requestContact((shared, res) => {
      if (!mounted.current || !shared) return;
      const raw = res?.responseUnsafe?.contact?.phone_number;
      if (!raw) return;
      const national = nationalPhoneFromShared(raw);
      setForeign(national === null);
      if (national) onPhone(national, typeof res?.response === "string" && res.response ? res.response : null);
    });
  }

  // Sits under the phone input as part of that field group: same radius and
  // border as the input, quieter than the form's primary button.
  return (
    <div className="space-y-1.5">
      <button
        type="button"
        onClick={share}
        className="flex min-h-11 w-full items-center justify-center gap-2 rounded-2xl border border-border bg-card px-4 text-sm font-bold text-foreground transition-colors hover:border-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:scale-[0.99]"
      >
        <Smartphone aria-hidden="true" className="h-4 w-4 shrink-0 text-accent-ink" />
        {t("sharePhone")}
      </button>
      {foreign && (
        <p role="alert" className="text-xs font-semibold text-danger">
          {t("phoneNotUzbek")}
        </p>
      )}
    </div>
  );
}
