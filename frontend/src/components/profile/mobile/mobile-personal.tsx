"use client";

import { useTranslations } from "next-intl";
import { Check } from "lucide-react";
import { MobileScreen } from "./mobile-screen";

type Props = {
  onBack: () => void;
  /** Already grouped for reading — "+998 90 111 00 22". */
  phone: string;
  name: string;
  region: string;
  onNameChange: (value: string) => void;
  onRegionChange: (value: string) => void;
  onSave: (e: React.FormEvent) => void;
  disabled: boolean;
  saving: boolean;
  saved: boolean;
  errorKey: "loadError" | "saveError" | null;
  onRetry: () => void;
};

/** The form id the bottom-pinned CTA points at — the button lives outside the form. */
const FORM_ID = "profile-mobile-personal-form";

export function MobilePersonal({
  onBack,
  phone,
  name,
  region,
  onNameChange,
  onRegionChange,
  onSave,
  disabled,
  saving,
  saved,
  errorKey,
  onRetry,
}: Props) {
  const t = useTranslations("Profile");

  return (
    <MobileScreen
      title={t("personalInfo")}
      onBack={onBack}
      footer={
        <button
          type="submit"
          form={FORM_ID}
          disabled={disabled || saving}
          className="btn-3d-primary flex min-h-[50px] w-full items-center justify-center rounded-xl font-display text-[19px] font-extrabold disabled:pointer-events-none disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {saving ? t("saving") : t("save")}
        </button>
      }
    >
      {errorKey && (
        <div
          role="alert"
          className="rounded-xl border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive"
        >
          <p>{t(errorKey)}</p>
          {errorKey === "loadError" && (
            <button
              type="button"
              onClick={onRetry}
              className="mt-2 min-h-11 rounded-xl border border-destructive/40 px-3 text-sm font-bold"
            >
              {t("retry")}
            </button>
          )}
        </div>
      )}

      {saved && (
        <div
          role="status"
          className="flex items-center gap-2 rounded-xl border border-success/50 bg-success/10 p-3 text-sm font-medium text-success"
        >
          <Check aria-hidden="true" className="h-4 w-4" /> {t("savedSuccess")}
        </div>
      )}

      <form id={FORM_ID} onSubmit={onSave} className="flex flex-col gap-3">
        <div>
          <label
            htmlFor="profile-mobile-phone"
            className="mb-1.5 block text-xs font-bold text-muted-foreground"
          >
            {t("phoneLabel")}
          </label>
          <input
            id="profile-mobile-phone"
            type="text"
            disabled
            value={phone}
            autoComplete="tel"
            className="field-input text-muted-foreground"
          />
        </div>

        <div>
          <label
            htmlFor="profile-mobile-name"
            className="mb-1.5 block text-xs font-bold text-muted-foreground"
          >
            {t("nameLabel")}
          </label>
          <input
            id="profile-mobile-name"
            type="text"
            value={name}
            onChange={(e) => onNameChange(e.target.value)}
            placeholder={t("namePlaceholder")}
            autoComplete="name"
            disabled={disabled}
            className="field-input"
          />
        </div>

        <div>
          <label
            htmlFor="profile-mobile-region"
            className="mb-1.5 block text-xs font-bold text-muted-foreground"
          >
            {t("regionLabel")}
          </label>
          <input
            id="profile-mobile-region"
            type="text"
            value={region}
            onChange={(e) => onRegionChange(e.target.value)}
            placeholder={t("regionPlaceholder")}
            autoComplete="address-level1"
            disabled={disabled}
            className="field-input"
          />
        </div>
      </form>

      <p className="text-xs leading-[18px] text-muted-foreground">{t("phoneLocked")}</p>
    </MobileScreen>
  );
}
