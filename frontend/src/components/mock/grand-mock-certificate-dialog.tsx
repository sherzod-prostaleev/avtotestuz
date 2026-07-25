"use client";

import { useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Award, Check, Share2, X } from "lucide-react";
import confetti from "canvas-confetti";
import { Button } from "@/components/ui/button";
import { certificateShareUrl, shareOrCopyCertificateLink } from "@/lib/certificate-share";

interface GrandMockCertificateDialogProps {
  open: boolean;
  onClose: () => void;
  score: number;
  total: number;
  shareCode?: string | null;
}

/**
 * Celebratory dialog shown once, right after a Grand Mock session finishes
 * with status "passed" — reward theater plus a persisted shareable id when
 * the backend issued one (U-35).
 */
export function GrandMockCertificateDialog({
  open,
  onClose,
  score,
  total,
  shareCode,
}: GrandMockCertificateDialogProps) {
  const t = useTranslations("GrandMock");
  const locale = useLocale();
  const [copied, setCopied] = useState(false);

  const shareUrl = useMemo(() => {
    if (!shareCode || typeof window === "undefined") return null;
    return certificateShareUrl(window.location.origin, locale, shareCode);
  }, [locale, shareCode]);

  useEffect(() => {
    if (!open) return;
    const duration = 2000;
    const end = Date.now() + duration;
    const frame = () => {
      confetti({
        particleCount: 3,
        angle: 60,
        spread: 60,
        origin: { x: 0, y: 0.7 },
        colors: ["#facc15", "#22c55e", "#3b82f6"],
      });
      confetti({
        particleCount: 3,
        angle: 120,
        spread: 60,
        origin: { x: 1, y: 0.7 },
        colors: ["#facc15", "#22c55e", "#3b82f6"],
      });
      if (Date.now() < end) requestAnimationFrame(frame);
    };
    frame();

    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open, onClose]);

  if (!open) return null;

  async function shareOrCopy() {
    if (!shareUrl) return;
    try {
      const mode = await shareOrCopyCertificateLink({
        url: shareUrl,
        title: t("certificateTitle"),
        text: t("certificateBody", { score, total }),
      });
      if (mode === "clipboard") {
        setCopied(true);
        window.setTimeout(() => setCopied(false), 2000);
      }
    } catch {
      /* abort / clipboard denied */
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={t("certificateTitle")}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4 backdrop-blur-sm"
    >
      <div className="relative w-full max-w-md rounded-3xl border border-gold/40 bg-card p-8 text-center">
        <button
          type="button"
          onClick={onClose}
          aria-label={t("certificateClose")}
          className="absolute right-3 top-3 flex h-10 w-10 items-center justify-center rounded-xl border border-border text-muted-foreground transition-colors hover:border-accent hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <X className="h-5 w-5" aria-hidden="true" />
        </button>

        <div className="mx-auto flex h-20 w-20 items-center justify-center rounded-full border border-gold/50 bg-gold/15">
          <Award className="h-11 w-11 text-gold" aria-hidden="true" />
        </div>

        <h2 className="mt-5 font-display text-2xl font-extrabold">{t("certificateTitle")}</h2>
        <p className="mx-auto mt-3 max-w-sm text-sm text-muted-foreground">
          {t("certificateBody", { score, total })}
        </p>

        {shareCode ? (
          <div className="mt-5 space-y-2 rounded-2xl border border-border bg-muted/40 px-4 py-3 text-left">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              {t("certificateShareLabel")}
            </p>
            <p className="font-mono text-sm break-all text-foreground">{shareCode}</p>
            {shareUrl ? (
              <Button type="button" variant="outline" size="sm" className="w-full gap-2" onClick={() => void shareOrCopy()}>
                {copied ? <Check className="h-4 w-4" aria-hidden /> : <Share2 className="h-4 w-4" aria-hidden />}
                {copied ? t("certificateCopied") : t("certificateShareOrCopy")}
              </Button>
            ) : null}
          </div>
        ) : null}

        <Button className="mt-6" variant="game" size="lg" onClick={onClose}>
          {t("certificateClose")}
        </Button>
      </div>
    </div>
  );
}
