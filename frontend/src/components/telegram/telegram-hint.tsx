"use client";

import { useEffect } from "react";
import { Info } from "lucide-react";
import { dismissTelegramHint, useTelegramHint } from "@/lib/telegram/hint";

// Long enough to read one line before Telegram's own sheet covers the page.
const HINT_MS = 6000;

/**
 * One-line note from lib/telegram/hint, pinned under Telegram's header. Polite
 * live region: it explains, it does not interrupt.
 */
export function TelegramHint() {
  const hint = useTelegramHint();

  useEffect(() => {
    if (!hint) return;
    const timer = window.setTimeout(() => dismissTelegramHint(hint.id), HINT_MS);
    return () => window.clearTimeout(timer);
  }, [hint]);

  if (!hint) return null;
  return (
    <div className="tg-hint pointer-events-none fixed inset-x-0 z-[90] flex justify-center px-4">
      <p
        role="status"
        aria-live="polite"
        className="flex max-w-sm animate-fade-in items-start gap-2 rounded-2xl border border-border bg-card px-4 py-3 text-sm font-semibold text-foreground shadow-lg motion-reduce:animate-none"
      >
        <Info aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0 text-accent-ink" />
        <span>{hint.text}</span>
      </p>
    </div>
  );
}
