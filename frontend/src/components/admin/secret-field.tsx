"use client";

import { useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AdminTooltip, type AdminTooltipContent } from "@/components/admin/admin-tooltip";

type SecretFieldProps = {
  maskedValue: string;
  label: string;
  tooltip: AdminTooltipContent;
  revealLabel: string;
  hideLabel: string;
  onReveal?: () => Promise<string | null>;
};

/** Masked secret — plaintext only via explicit reveal callback (once). */
export function SecretField({
  maskedValue,
  label,
  tooltip,
  revealLabel,
  hideLabel,
  onReveal,
}: SecretFieldProps) {
  const [shown, setShown] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function toggle() {
    if (shown) {
      setShown(null);
      return;
    }
    if (!onReveal) return;
    setBusy(true);
    try {
      const v = await onReveal();
      if (v) setShown(v);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-1.5">
      <div className="flex items-center gap-1">
        <span className="text-xs font-semibold text-muted-foreground">{label}</span>
        <AdminTooltip content={tooltip} />
      </div>
      <div className="flex items-center gap-2">
        <code className="flex-1 truncate rounded-xl border border-border bg-background px-3 py-2 font-mono text-xs">
          {shown ?? maskedValue}
        </code>
        {onReveal ? (
          <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void toggle()}>
            {shown ? (
              <>
                <EyeOff className="mr-1 h-3.5 w-3.5" aria-hidden /> {hideLabel}
              </>
            ) : (
              <>
                <Eye className="mr-1 h-3.5 w-3.5" aria-hidden /> {revealLabel}
              </>
            )}
          </Button>
        ) : null}
      </div>
    </div>
  );
}
