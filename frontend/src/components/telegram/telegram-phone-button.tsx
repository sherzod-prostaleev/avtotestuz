"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTelegram } from "@/components/telegram/telegram-provider";
import { nationalPhoneFromShared } from "@/lib/phone-format";

/**
 * Telegram's own "share your number" sheet. Only pre-fills the field: the
 * number still goes through the same validation and password check as typing
 * it (spec D3), so this adds no new trust.
 */
export function TelegramPhoneButton({ onPhone }: { onPhone: (national: string) => void }) {
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
    webApp!.requestContact((shared, res) => {
      if (!mounted.current || !shared) return;
      const raw = res?.responseUnsafe?.contact?.phone_number;
      if (!raw) return;
      const national = nationalPhoneFromShared(raw);
      setForeign(national === null);
      if (national) onPhone(national);
    });
  }

  return (
    <div className="space-y-2">
      <Button type="button" variant="outline" size="lg" className="min-h-11 w-full text-sm font-extrabold" onClick={share}>
        <Send aria-hidden="true" className="mr-2 h-4 w-4" /> {t("sharePhone")}
      </Button>
      {foreign && (
        <p role="alert" className="text-xs font-semibold text-danger">
          {t("phoneNotUzbek")}
        </p>
      )}
    </div>
  );
}
