"use client";

import { useEffect, useRef } from "react";
import { useTranslations } from "next-intl";
import { Loader2, Unlink } from "lucide-react";

/**
 * «Telegramni uzish» with an inline confirm step, shared by the wide card and
 * the phone panel. Inline rather than a dialog: one sentence and two buttons
 * fit where the action was, and focus never has to travel. Ink colours, not
 * the base red: those hold 4.5:1 for small bold text in both themes.
 */
export function TelegramUnlink({
  confirming,
  unlinking,
  onAsk,
  onCancel,
  onConfirm,
}: {
  confirming: boolean;
  unlinking: boolean;
  onAsk: () => void;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const t = useTranslations("TelegramLink");
  // The ask button disappears under the keyboard/screen-reader focus; move
  // it to the safe choice instead of dropping it on <body>.
  const cancelRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (confirming) cancelRef.current?.focus();
  }, [confirming]);

  if (!confirming) {
    return (
      <button
        type="button"
        onClick={onAsk}
        className="inline-flex min-h-11 items-center gap-2 rounded-xl px-1 text-sm font-bold text-danger-ink focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <Unlink aria-hidden="true" className="h-4 w-4 shrink-0" />
        {t("unlinkButton")}
      </button>
    );
  }

  return (
    <div className="space-y-3 rounded-2xl border border-destructive/40 bg-destructive/5 p-3">
      <p className="text-sm font-semibold leading-snug text-foreground">{t("unlinkConfirm")}</p>
      <div className="flex gap-2">
        <button
          ref={cancelRef}
          type="button"
          onClick={onCancel}
          disabled={unlinking}
          className="flex min-h-11 flex-1 items-center justify-center rounded-xl border border-border bg-card px-3 text-sm font-bold disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {t("cancel")}
        </button>
        <button
          type="button"
          onClick={onConfirm}
          disabled={unlinking}
          className="flex min-h-11 flex-1 items-center justify-center gap-2 rounded-xl border border-destructive/60 bg-card px-3 text-sm font-bold text-danger-ink disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {unlinking && <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin motion-reduce:animate-none" />}
          {unlinking ? t("unlinking") : t("unlinkYes")}
        </button>
      </div>
    </div>
  );
}
