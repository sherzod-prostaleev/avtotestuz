"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import {
  ChevronRight,
  CreditCard,
  Gift,
  Lock,
  LogOut,
  MapPin,
  Send,
  User,
} from "lucide-react";
import { ChangePasswordForm } from "@/components/profile/change-password-form";
import { PaymentHistoryCard } from "@/components/profile/payment-history-card";
import { ReferralCard } from "@/components/profile/referral-card";
import { TelegramLinkCard } from "@/components/profile/telegram-link-card";
import { MobilePersonal } from "./mobile-personal";
import { MobileScreen } from "./mobile-screen";

type Panel = "personal" | "password" | "telegram" | "payments" | "referral";

export interface ProfileMobileProps {
  name: string;
  region: string;
  phone: string;
  referralCode: string;
  isVip: boolean;
  onNameChange: (value: string) => void;
  onRegionChange: (value: string) => void;
  onSave: (event: React.FormEvent) => void;
  saving: boolean;
  saved: boolean;
  errorKey: "loadError" | "saveError" | null;
  onRetry: () => void;
  onLogout: () => void;
  loading: boolean;
  className?: string;
}

/**
 * The phone profile: a settings list whose rows open sub-screens, rather than
 * the wide page's column of eight stacked cards.
 *
 * The sub-screens are panels held in this component's own state, not routes.
 * The ≥768px design is one frozen column at `/profile`; six real routes would
 * each need a wide design of their own, or would show a phone-shaped screen to
 * a desktop visitor. A panel keeps the wide DOM untouched and needs no refetch.
 *
 * Only one panel is mounted at a time — never all six behind CSS — so a test
 * querying by text finds one of a thing, not six.
 */
export function ProfileMobile({
  name,
  region,
  phone,
  referralCode,
  isVip,
  onNameChange,
  onRegionChange,
  onSave,
  saving,
  saved,
  errorKey,
  onRetry,
  onLogout,
  loading,
  className = "",
}: ProfileMobileProps) {
  const t = useTranslations("Profile");
  const [panel, setPanel] = useState<Panel | null>(null);
  const close = () => setPanel(null);

  if (panel === "personal") {
    return (
      <div className={className}>
        <MobilePersonal
          onBack={close}
          phone={phone}
          name={name}
          region={region}
          onNameChange={onNameChange}
          onRegionChange={onRegionChange}
          onSave={onSave}
          disabled={loading}
          saving={saving}
          saved={saved}
          errorKey={errorKey}
          onRetry={onRetry}
        />
      </div>
    );
  }

  if (panel) {
    const titles: Record<Exclude<Panel, "personal">, string> = {
      password: t("passwordTitle"),
      telegram: t("telegramTitle"),
      payments: t("paymentsTitle"),
      referral: t("referralTitle"),
    };
    return (
      <div className={className}>
        <MobileScreen title={titles[panel]} onBack={close}>
          {panel === "password" && <ChangePasswordForm bare onSuccess={close} />}
          {panel === "telegram" && <TelegramLinkCard />}
          {panel === "payments" && <PaymentHistoryCard />}
          {panel === "referral" && <ReferralCard />}
        </MobileScreen>
      </div>
    );
  }

  const row = (
    key: string,
    Icon: typeof User,
    label: string,
    value: string | null,
    onClick: () => void,
    first: boolean,
  ) => (
    <button
      key={key}
      type="button"
      onClick={onClick}
      className={`flex min-h-12 w-full items-center gap-3 px-3.5 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${
        first ? "" : "border-t border-border"
      }`}
    >
      <Icon aria-hidden="true" className="h-5 w-5 shrink-0 text-muted-foreground" />
      <span className="min-w-0 flex-1 truncate text-sm">{label}</span>
      {value && <span className="shrink-0 truncate text-sm font-bold">{value}</span>}
      <ChevronRight aria-hidden="true" className="h-4 w-4 shrink-0 text-muted-foreground" />
    </button>
  );

  const group = (label: string, rows: React.ReactNode) => (
    <div>
      <p className="mb-1 text-xs font-extrabold uppercase leading-none tracking-[0.08em] text-muted-foreground">
        {label}
      </p>
      <div className="overflow-hidden rounded-2xl border border-border bg-card">{rows}</div>
    </div>
  );

  return (
    <div className={`flex flex-col gap-2 ${className}`}>
      <div className="surface-raised flex items-center gap-3 rounded-2xl border border-border bg-card p-3">
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-accent/15 font-display text-lg font-extrabold text-accent">
          {(name || phone).trim().charAt(0).toUpperCase()}
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate font-display text-lg font-extrabold">{name || t("noName")}</p>
          <p className="truncate text-xs text-muted-foreground">{phone}</p>
        </div>
        {isVip && (
          <span className="inline-flex h-[22px] shrink-0 items-center gap-1 rounded-full bg-gold/15 px-2 text-xs font-extrabold text-gold">
            VIP
          </span>
        )}
      </div>

      {group(
        t("accountGroup"),
        <>
          {row("name", User, t("nameLabel"), name || null, () => setPanel("personal"), true)}
          {row("region", MapPin, t("regionLabel"), region || null, () => setPanel("personal"), false)}
          {row("password", Lock, t("passwordTitle"), null, () => setPanel("password"), false)}
        </>,
      )}

      {group(
        t("connectionsGroup"),
        row("telegram", Send, t("telegramTitle"), null, () => setPanel("telegram"), true),
      )}

      {group(
        t("paymentGroup"),
        <>
          {row("payments", CreditCard, t("paymentsTitle"), null, () => setPanel("payments"), true)}
          {row("referral", Gift, t("referralTitle"), referralCode || null, () => setPanel("referral"), false)}
        </>,
      )}

      <button
        type="button"
        onClick={onLogout}
        className="flex min-h-12 items-center gap-3 rounded-2xl border border-border bg-card px-3.5 text-left text-destructive focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <LogOut aria-hidden="true" className="h-5 w-5 shrink-0" />
        <span className="min-w-0 flex-1 truncate text-sm font-bold">{t("logout")}</span>
      </button>
    </div>
  );
}
