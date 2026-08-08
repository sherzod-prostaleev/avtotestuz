"use client";

import { useState } from "react";
import { Check, Copy, KeyRound } from "lucide-react";

type Props = {
  userId: string;
  issueLabel: string;
  copyLabel: string;
  copiedLabel: string;
  issuedHint: string;
  errorLabel: string;
};

/**
 * Issues a one-time temporary password via admin API.
 * Plaintext is shown only in-memory from the response — never loaded from DB.
 */
export function TemporaryPasswordButton({
  userId,
  issueLabel,
  copyLabel,
  copiedLabel,
  issuedHint,
  errorLabel,
}: Props) {
  const [password, setPassword] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function issue() {
    setBusy(true);
    setError(null);
    setCopied(false);
    try {
      const res = await fetch(`/api/admin/users/${userId}/temporary-password`, {
        method: "POST",
      });
      if (!res.ok) {
        setError(errorLabel);
        setPassword(null);
        return;
      }
      const json = await res.json();
      const data = json.data ?? json;
      const next = String(data.temporary_password ?? "");
      if (!next) {
        setError(errorLabel);
        return;
      }
      setPassword(next);
    } catch {
      setError(errorLabel);
      setPassword(null);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-2">
      <button
        type="button"
        disabled={busy}
        onClick={() => void issue()}
        className="inline-flex w-full items-center justify-center gap-2 rounded-lg bg-primary px-3 py-2 text-sm font-semibold text-primary-foreground disabled:opacity-50"
      >
        <KeyRound className="h-4 w-4" />
        {issueLabel}
      </button>
      {error ? <p className="text-xs text-destructive">{error}</p> : null}
      {password ? (
        <div className="space-y-1">
          <p className="text-xs text-muted-foreground">{issuedHint}</p>
          <div className="flex items-center gap-2 rounded-lg border border-border bg-muted/40 px-3 py-2">
            <code className="min-w-0 flex-1 truncate font-mono text-sm">{password}</code>
            <button
              type="button"
              className="inline-flex shrink-0 items-center gap-1 rounded-md border border-border px-2 py-1 text-xs font-semibold"
              onClick={async () => {
                try {
                  await navigator.clipboard.writeText(password);
                  setCopied(true);
                  window.setTimeout(() => setCopied(false), 1500);
                } catch {
                  /* ignore */
                }
              }}
            >
              {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
              {copied ? copiedLabel : copyLabel}
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
