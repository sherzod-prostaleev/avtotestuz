"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Check, Lock } from "lucide-react";
import { ApiError, apiPost } from "@/lib/api-client";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";

type Props = {
  /** When true, hide the card chrome (used on the mandatory change page). */
  bare?: boolean;
  onSuccess?: () => void;
};

const ERROR_KEYS: Record<string, string> = {
  invalid_current_password: "passwordErrorCurrent",
  password_mismatch: "passwordErrorMismatch",
  weak_password: "passwordErrorWeak",
  password_unchanged: "passwordErrorUnchanged",
  password_not_set: "passwordErrorNotSet",
  network_error: "passwordErrorNetwork",
};

export function ChangePasswordForm({ bare = false, onSuccess }: Props) {
  const t = useTranslations("Profile");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [success, setSuccess] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErrorKey(null);
    setSuccess(false);

    if (newPassword.length < 8) {
      setErrorKey("passwordErrorWeak");
      return;
    }
    if (newPassword !== confirmPassword) {
      setErrorKey("passwordErrorMismatch");
      return;
    }

    setSubmitting(true);
    try {
      await apiPost("me/password", {
        current_password: currentPassword,
        new_password: newPassword,
        confirm_password: confirmPassword,
      });
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setSuccess(true);
      onSuccess?.();
    } catch (err) {
      if (err instanceof ApiError) {
        setErrorKey(ERROR_KEYS[err.code] ?? "passwordErrorUnknown");
      } else {
        setErrorKey("passwordErrorNetwork");
      }
    } finally {
      setSubmitting(false);
    }
  }

  const form = (
    <form onSubmit={handleSubmit} className="space-y-4" autoComplete="off">
      <div>
        <label htmlFor="current-password" className="mb-1.5 block text-xs font-bold text-muted-foreground">
          {t("passwordCurrent")}
        </label>
        <input
          id="current-password"
          type="password"
          autoComplete="current-password"
          value={currentPassword}
          onChange={(e) => {
            setCurrentPassword(e.target.value);
            setErrorKey(null);
            setSuccess(false);
          }}
          className="field-input"
          required
        />
      </div>
      <div>
        <label htmlFor="new-password" className="mb-1.5 block text-xs font-bold text-muted-foreground">
          {t("passwordNew")}
        </label>
        <input
          id="new-password"
          type="password"
          autoComplete="new-password"
          value={newPassword}
          onChange={(e) => {
            setNewPassword(e.target.value);
            setErrorKey(null);
            setSuccess(false);
          }}
          className="field-input"
          required
          minLength={8}
        />
      </div>
      <div>
        <label htmlFor="confirm-password" className="mb-1.5 block text-xs font-bold text-muted-foreground">
          {t("passwordConfirm")}
        </label>
        <input
          id="confirm-password"
          type="password"
          autoComplete="new-password"
          value={confirmPassword}
          onChange={(e) => {
            setConfirmPassword(e.target.value);
            setErrorKey(null);
            setSuccess(false);
          }}
          className="field-input"
          required
          minLength={8}
        />
      </div>

      {errorKey && (
        <div role="alert" className="rounded-xl border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
          {t(errorKey)}
        </div>
      )}

      {success && (
        <div
          role="status"
          className="flex items-center gap-2 rounded-xl border border-success/50 bg-success/10 p-3 text-sm font-medium text-success"
        >
          <Check aria-hidden="true" className="h-4 w-4" /> {t("passwordSuccess")}
        </div>
      )}

      <Button type="submit" variant="game" size="sm" className="w-full sm:w-auto" disabled={submitting}>
        {submitting ? t("passwordSaving") : t("passwordSubmit")}
      </Button>
    </form>
  );

  if (bare) {
    return form;
  }

  return (
    <Card className="p-5 sm:p-6">
      <CardHeader className="mb-4 flex flex-row items-center gap-2 p-0">
        <Lock aria-hidden="true" className="h-5 w-5 text-accent" />
        <CardTitle className="text-base font-bold">{t("passwordSection")}</CardTitle>
      </CardHeader>
      <p className="mb-4 text-xs text-muted-foreground">{t("passwordHint")}</p>
      {form}
    </Card>
  );
}
