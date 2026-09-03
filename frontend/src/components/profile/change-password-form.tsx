"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Check, Eye, EyeOff, Lock, ShieldCheck } from "lucide-react";
import { ApiError, apiPost } from "@/lib/api-client";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";

type Props = {
  /** When true, hide the card chrome (used on the mandatory change page). */
  bare?: boolean;
  /**
   * The phone profile screen's variant: a per-field eye toggle, the "we never
   * store your password" note, and a full-width CTA pushed to the bottom edge.
   * Off everywhere else, so the desktop card and the mandatory-change page keep
   * exactly the form they have today.
   */
  reveal?: boolean;
  onSuccess?: () => void;
};

type PasswordField = "current" | "next" | "confirm";

const ERROR_KEYS: Record<string, string> = {
  invalid_current_password: "passwordErrorCurrent",
  password_mismatch: "passwordErrorMismatch",
  weak_password: "passwordErrorWeak",
  password_unchanged: "passwordErrorUnchanged",
  password_not_set: "passwordErrorNotSet",
  network_error: "passwordErrorNetwork",
};

/**
 * One password row. With `reveal` off it renders exactly the input the wide
 * card has always rendered — no wrapper, no button — so the desktop form and
 * the mandatory-change page are untouched. With it on, the artboard's eye
 * toggle sits inside the field.
 */
function PasswordRow({
  id,
  label,
  value,
  onChange,
  autoComplete,
  minLength,
  reveal,
  shown,
  onToggle,
  showLabel,
  hideLabel,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  autoComplete: string;
  minLength?: number;
  reveal: boolean;
  shown: boolean;
  onToggle: () => void;
  showLabel: string;
  hideLabel: string;
}) {
  const input = (
    <input
      id={id}
      type={reveal && shown ? "text" : "password"}
      autoComplete={autoComplete}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className={reveal ? "field-input pr-12" : "field-input"}
      required
      minLength={minLength}
    />
  );

  return (
    <div>
      <label htmlFor={id} className="mb-1.5 block text-xs font-bold text-muted-foreground">
        {label}
      </label>
      {reveal ? (
        <div className="relative">
          {input}
          <button
            type="button"
            onClick={onToggle}
            aria-label={shown ? hideLabel : showLabel}
            className="absolute right-0 top-1/2 flex h-11 w-11 -translate-y-1/2 items-center justify-center text-muted-foreground"
          >
            {shown ? (
              <EyeOff aria-hidden="true" className="h-5 w-5" />
            ) : (
              <Eye aria-hidden="true" className="h-5 w-5" />
            )}
          </button>
        </div>
      ) : (
        input
      )}
    </div>
  );
}

export function ChangePasswordForm({ bare = false, reveal = false, onSuccess }: Props) {
  const t = useTranslations("Profile");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [success, setSuccess] = useState(false);
  const [shown, setShown] = useState<Record<PasswordField, boolean>>({
    current: false,
    next: false,
    confirm: false,
  });
  const toggle = (field: PasswordField) =>
    setShown((prev) => ({ ...prev, [field]: !prev[field] }));
  const clearFeedback = () => {
    setErrorKey(null);
    setSuccess(false);
  };

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
    <form
      onSubmit={handleSubmit}
      className={reveal ? "flex flex-1 flex-col gap-3" : "space-y-4"}
      autoComplete="off"
    >
      <PasswordRow
        id="current-password"
        label={t("passwordCurrent")}
        value={currentPassword}
        onChange={(value) => {
          setCurrentPassword(value);
          clearFeedback();
        }}
        autoComplete="current-password"
        reveal={reveal}
        shown={shown.current}
        onToggle={() => toggle("current")}
        showLabel={t("passwordShow")}
        hideLabel={t("passwordHide")}
      />
      <PasswordRow
        id="new-password"
        label={t("passwordNew")}
        value={newPassword}
        onChange={(value) => {
          setNewPassword(value);
          clearFeedback();
        }}
        autoComplete="new-password"
        minLength={8}
        reveal={reveal}
        shown={shown.next}
        onToggle={() => toggle("next")}
        showLabel={t("passwordShow")}
        hideLabel={t("passwordHide")}
      />
      <PasswordRow
        id="confirm-password"
        label={t("passwordConfirm")}
        value={confirmPassword}
        onChange={(value) => {
          setConfirmPassword(value);
          clearFeedback();
        }}
        autoComplete="new-password"
        minLength={8}
        reveal={reveal}
        shown={shown.confirm}
        onToggle={() => toggle("confirm")}
        showLabel={t("passwordShow")}
        hideLabel={t("passwordHide")}
      />

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

      {reveal ? (
        <>
          <div className="flex items-start gap-2.5 rounded-2xl border border-border bg-card p-3">
            <ShieldCheck aria-hidden="true" className="mt-0.5 h-5 w-5 shrink-0 text-success" />
            <p className="min-w-0 flex-1 text-[13px] leading-snug text-muted-foreground">
              {t("passwordNeverStored")}
            </p>
          </div>
          <div className="flex-1" />
          <button
            type="submit"
            disabled={submitting}
            className="btn-3d-primary inline-flex min-h-[50px] w-full items-center justify-center rounded-xl px-4 font-display text-lg font-extrabold disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {submitting ? t("passwordSaving") : t("passwordSubmit")}
          </button>
        </>
      ) : (
        <Button type="submit" variant="game" size="sm" className="w-full sm:w-auto" disabled={submitting}>
          {submitting ? t("passwordSaving") : t("passwordSubmit")}
        </Button>
      )}
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
