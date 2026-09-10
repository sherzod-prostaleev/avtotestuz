"use client";

import { useCallback, useEffect, useId, useRef, type ReactNode } from "react";
import { AnimatePresence, motion } from "motion/react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";

export interface ConfirmDialogProps {
  open: boolean;
  title: string;
  /** Body copy. A node, not a string, so callers can emphasise a count. */
  description: ReactNode;
  confirmLabel: string;
  cancelLabel: string;
  onConfirm: () => void;
  onCancel: () => void;
  /** Disables both buttons and reads on the confirm one while a request runs. */
  busy?: boolean;
  busyLabel?: string;
  /** Icon shown beside the title. */
  icon?: ReactNode;
}

/**
 * A modal that asks before something irreversible happens.
 *
 * The look follows the sign-detail modal on /signs — a bottom sheet on a
 * phone, a centred card from `sm:` up — so it is not a second dialog language
 * in the same app. What it adds is the keyboard and focus behaviour a
 * destructive confirmation needs and a browsable detail sheet does not:
 *
 *  - Focus moves to Cancel on open, so Enter on a keyboard or the OK button on
 *    a classroom TV remote lands on the safe choice, never on the destructive
 *    one.
 *  - Tab cycles inside the dialog. Without this the next Tab reaches the page
 *    behind the backdrop, where a click cannot follow it.
 *  - Escape cancels, and focus returns to whatever opened the dialog.
 *  - The page behind cannot scroll while it is open.
 *
 * `window.confirm` would give some of this for free and is deliberately not
 * used: a native modal dialog blocks the kiosk's browser-automation channel
 * and, on the station PCs, leaves a screen nobody can dismiss remotely.
 */
export function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel,
  cancelLabel,
  onConfirm,
  onCancel,
  busy = false,
  busyLabel,
  icon,
}: ConfirmDialogProps) {
  const titleId = useId();
  const descriptionId = useId();
  const panelRef = useRef<HTMLDivElement>(null);
  const cancelRef = useRef<HTMLButtonElement>(null);
  // Captured on open, restored on close: after a dialog the caret belongs back
  // on the control that opened it, not at the top of the document.
  const openerRef = useRef<HTMLElement | null>(null);

  // Cancelling must stay callable from an effect without re-arming that effect
  // on every parent render.
  const onCancelRef = useRef(onCancel);
  useEffect(() => {
    onCancelRef.current = onCancel;
  }, [onCancel]);

  useEffect(() => {
    if (!open) return;
    openerRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    // Focus after paint: the panel is not in the DOM during this render pass.
    const raf = requestAnimationFrame(() => cancelRef.current?.focus());
    return () => {
      cancelAnimationFrame(raf);
      openerRef.current?.focus();
    };
  }, [open]);

  // Scroll lock. The previous inline value is restored rather than cleared, so
  // this cannot quietly unlock a page some other component had locked.
  useEffect(() => {
    if (!open) return;
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previous;
    };
  }, [open]);

  const handleKeyDown = useCallback((event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Escape") {
      event.preventDefault();
      onCancelRef.current();
      return;
    }
    if (event.key !== "Tab") return;

    const panel = panelRef.current;
    if (!panel) return;
    const focusable = Array.from(
      panel.querySelectorAll<HTMLElement>(
        'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
      )
    );
    if (focusable.length === 0) return;

    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    const active = document.activeElement;
    // Wrap at both ends. The `!panel.contains(active)` arm matters when every
    // control is disabled mid-request and focus has fallen back to <body>.
    if (event.shiftKey && (active === first || !panel.contains(active))) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && (active === last || !panel.contains(active))) {
      event.preventDefault();
      first.focus();
    }
  }, []);

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.18 }}
          onKeyDown={handleKeyDown}
          // The backdrop dismisses, but only when the click starts and ends on
          // the backdrop itself: a drag that begins inside the panel and ends
          // outside it is a text selection, not a dismissal.
          onMouseDown={(event) => {
            if (event.target === event.currentTarget && !busy) onCancel();
          }}
          className="fixed inset-0 z-[60] flex items-end justify-center bg-black/60 p-0 backdrop-blur-sm sm:items-center sm:p-4"
        >
          <motion.div
            ref={panelRef}
            role="dialog"
            aria-modal="true"
            aria-labelledby={titleId}
            aria-describedby={descriptionId}
            initial={{ opacity: 0, scale: 0.96, y: 16 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 16 }}
            transition={{ duration: 0.22, ease: [0.16, 1, 0.3, 1] }}
            className="w-full max-w-md"
          >
            {/* As a bottom sheet, the last button sits on the very edge of the
                screen, which on a phone with a home indicator is not a place a
                thumb can reach. The centred variant from `sm:` up needs no
                such allowance. */}
            <Card className="max-h-[92dvh] w-full space-y-4 overflow-y-auto rounded-b-none rounded-t-3xl p-5 pb-[calc(1.25rem+env(safe-area-inset-bottom))] sm:rounded-2xl sm:p-6 sm:pb-6">
              <div className="flex items-start gap-3">
                {icon && (
                  <span
                    aria-hidden="true"
                    className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-danger/15 text-danger"
                  >
                    {icon}
                  </span>
                )}
                <h2 id={titleId} className="font-display text-lg font-extrabold leading-snug sm:text-xl">
                  {title}
                </h2>
              </div>

              <div id={descriptionId} className="space-y-1.5 text-sm leading-relaxed text-muted-foreground">
                {description}
              </div>

              {/* Stacked on a phone with the destructive action on top and the
                  way out beneath it; reversed from `sm:` up, where confirming
                  conventionally sits on the right. Initial focus is placed on
                  Cancel explicitly, so this ordering never decides it. */}
              <div className="flex flex-col gap-2 pt-1 sm:flex-row-reverse sm:justify-start sm:gap-3">
                <Button
                  type="button"
                  variant="destructive"
                  className="w-full sm:w-auto"
                  onClick={onConfirm}
                  disabled={busy}
                >
                  {busy && busyLabel ? busyLabel : confirmLabel}
                </Button>
                <Button
                  ref={cancelRef}
                  type="button"
                  variant="outline"
                  className="w-full sm:w-auto"
                  onClick={onCancel}
                  disabled={busy}
                >
                  {cancelLabel}
                </Button>
              </div>
            </Card>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
