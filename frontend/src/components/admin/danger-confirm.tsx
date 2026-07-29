"use client";

import { useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { AlertTriangle, X } from "lucide-react";
import { Button } from "@/components/ui/button";

type DangerConfirmProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  /** Warning lines shown with ⚠ */
  warnings: string[];
  /** Phrase the operator must type to confirm */
  confirmPhrase: string;
  confirmPhraseLabel: string;
  confirmLabel: string;
  cancelLabel: string;
  busy?: boolean;
  onConfirm: () => void | Promise<void>;
};

export function DangerConfirm({
  open,
  onOpenChange,
  title,
  warnings,
  confirmPhrase,
  confirmPhraseLabel,
  confirmLabel,
  cancelLabel,
  busy,
  onConfirm,
}: DangerConfirmProps) {
  const [typed, setTyped] = useState("");
  const match = typed.trim() === confirmPhrase;

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(v) => {
        if (!v) setTyped("");
        onOpenChange(v);
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-overlay/70 backdrop-blur-[2px]" />
        <Dialog.Content
          className="fixed left-1/2 top-1/2 z-50 w-[min(440px,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2 rounded-2xl border border-destructive/40 bg-card p-5 shadow-2xl focus:outline-none"
          aria-describedby="danger-confirm-warnings"
        >
          <div className="flex items-start justify-between gap-3">
            <div className="flex items-start gap-2">
              <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-destructive" aria-hidden />
              <Dialog.Title className="font-display text-lg font-black tracking-tight">
                {title}
              </Dialog.Title>
            </div>
            <Dialog.Close asChild>
              <button
                type="button"
                className="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-lg p-1 text-muted-foreground hover:bg-foreground/[0.06] hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                aria-label={cancelLabel}
              >
                <X className="h-4 w-4" />
              </button>
            </Dialog.Close>
          </div>
          <ul id="danger-confirm-warnings" className="mt-3 space-y-1.5 text-sm text-warning">
            {warnings.map((w) => (
              <li key={w}>⚠ {w}</li>
            ))}
          </ul>
          <label className="mt-4 block space-y-1.5">
            <span className="text-xs font-semibold text-muted-foreground">
              {confirmPhraseLabel}{" "}
              <span className="font-mono text-foreground">{confirmPhrase}</span>
            </span>
            <input
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              autoComplete="off"
              className="h-10 w-full rounded-xl border border-border bg-background px-3 font-mono text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            />
          </label>
          <div className="mt-4 flex justify-end gap-2">
            <Button
              type="button"
              size="sm"
              variant="outline"
              disabled={busy}
              onClick={() => onOpenChange(false)}
            >
              {cancelLabel}
            </Button>
            <Button
              type="button"
              size="sm"
              variant="destructive"
              disabled={!match || busy}
              onClick={() => void onConfirm()}
            >
              {confirmLabel}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
