"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { BrandLogo } from "@/components/brand/brand-logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { ArrowLeft, KeyRound, Loader2, Lock, Phone, Send, ShieldCheck } from "lucide-react";
import { TelegramLogin, type TelegramLoginHandle } from "@/components/auth/telegram-login";
import { applyPendingReferralCode, capturePendingReferralCodeFromUrl } from "@/lib/referral-storage";
import { migrateDemoProgressOnLogin } from "@/lib/demo-progress-storage";
import { TelegramPhoneButton } from "@/components/telegram/telegram-phone-button";
import { useTelegram, useTelegramStatus } from "@/components/telegram/telegram-provider";
import { afterTelegramAuth, withTelegramInitData } from "@/lib/telegram/auth-body";
import { forgetNeedPhone } from "@/lib/telegram/need-phone-cache";
import { carryNextQuery, miniAppNext, safeNextPath } from "@/lib/telegram/safe-next";
import { formatNationalPhone, normalizeNationalPhone } from "@/lib/phone-format";

const ERROR_MESSAGE_KEYS: Record<string, string> = {
  invalid_phone: "errorInvalidPhone",
  invalid_credentials: "errorInvalidCredentials",
  account_blocked: "errorAccountBlocked",
  password_not_set: "errorPasswordNotSet",
  rate_limited: "errorRateLimited",
  network_error: "errorNetwork",
  weak_password: "errorWeakPassword",
};

const BRAND_CLASS =
  "flex min-w-0 items-center gap-2 font-display text-lg font-black text-foreground sm:gap-2.5 sm:text-xl";

/** National 9-digit UZ mobile (strips optional 998 country code). */
function normalizePhone(input: string): string {
  return normalizeNationalPhone(input);
}

export default function LoginPage() {
  const t = useTranslations("Login");
  const locale = useLocale();
  const router = useRouter();
  const webApp = useTelegram();
  const tgStatus = useTelegramStatus();
  const tgT = useTranslations("TelegramApp");
  const waitingForTelegram = tgStatus === "loading";
  // "off" only on the website (and the server render); any other status
  // means Telegram launched us, even before or without a working SDK.
  const inMiniApp = tgStatus !== "off";
  // Mini App only: a /tg deep link's target, carried across login ↔ register.
  const [nextParam, setNextParam] = useState<string | null>(null);
  // Same condition as the redirect after sign-in (miniAppNext): a link must
  // not promise a target the other form would then drop.
  const carryNext = carryNextQuery(webApp, nextParam, locale);
  const [phone, setPhone] = useState("");
  // Telegram's signed share of the phone (Mini App only): the link proof.
  const [tgContact, setTgContact] = useState<string | null>(null);
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [sessionExpired, setSessionExpired] = useState(false);
  const telegramLogin = useRef<TelegramLoginHandle>(null);
  const tgLoginT = useTranslations("TelegramLogin");

  useEffect(() => {
    capturePendingReferralCodeFromUrl();
    // Read the flag off location instead of useSearchParams: that hook opts
    // the whole page into a client-render bailout at build time, which is a
    // steep price for one line of reassurance. SessionExpiredGate sets it.
    try {
      const query = new URLSearchParams(window.location.search);
      setSessionExpired(query.get("expired") === "1");
      setNextParam(query.get("next"));
    } catch {
      /* a malformed query string just means no notice */
    }
  }, []);

  // honourNext: the website Telegram login returns to `?next=` (validated by
  // safeNextPath) like the Mini App does; the password form keeps its landing.
  async function finishAuth(mustChangePassword: boolean, honourNext = false) {
    // Side-effects must never block a successful login — cookies are already set.
    try {
      await applyPendingReferralCode();
    } catch {
      /* best-effort */
    }
    try {
      await migrateDemoProgressOnLogin();
    } catch {
      /* best-effort */
    }
    if (mustChangePassword) {
      router.push(`/${locale}/change-password`);
      return;
    }
    // A Mini App deep link (/tg?next=…) passed its target through the
    // welcome screen. The website keeps its old landing: the dashboard.
    const rawNext = new URLSearchParams(window.location.search).get("next");
    const next = honourNext ? rawNext : miniAppNext(webApp, rawNext);
    router.push(safeNextPath(next, locale));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    // The disabled button does not stop an Enter-key submit; without tg_init_data
    // the BFF would issue lax cookies the Mini App iframe drops.
    if (waitingForTelegram) return;
    setError(null);
    const localPhone = normalizePhone(phone);
    if (localPhone.length !== 9) {
      setError("invalid_phone");
      return;
    }
    if (password.length < 8) {
      setError("weak_password");
      return;
    }
    setSubmitting(true);
    try {
      let res: Response;
      try {
        res = await fetch("/api/auth/login", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(withTelegramInitData({ phone: localPhone, password }, webApp, tgContact)),
        });
      } catch {
        setError("network_error");
        return;
      }

      let code = "unknown";
      let mustChangePassword = false;
      let linked = false;
      try {
        const json = (await res.json()) as {
          error?: { code?: string };
          data?: { must_change_password?: boolean; telegram_linked?: boolean };
        };
        code = json.error?.code ?? "unknown";
        mustChangePassword = json.data?.must_change_password === true;
        linked = json.data?.telegram_linked === true;
      } catch {
        if (!res.ok) {
          setError("network_error");
          return;
        }
      }

      if (!res.ok) {
        setError(code === "unknown" && res.status >= 500 ? "network_error" : code);
        return;
      }
      // Fire and forget: CloudStorage and Telegram's phone sheet must never
      // hold up sign-in. No shared number yet → offer the sheet once; one
      // that was shared and still did not link would only be repeated.
      if (webApp) forgetNeedPhone();
      void afterTelegramAuth(linked, {
        webApp,
        askForContact: !tgContact && !mustChangePassword,
        explain: tgT("shareAfterLogin"),
      }).catch(() => {});
      await finishAuth(mustChangePassword);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div
      className="asphalt-hero flex min-h-screen flex-col bg-background auth-safe-bottom"
    >
      <header
        className="flex h-14 items-center justify-between border-b border-border px-3 sm:px-4 auth-safe-top"
      >
        {/* In the Mini App the landing page is a dead end with no way back to
            the app: the logo is not a link there. */}
        {inMiniApp ? (
          <span className={BRAND_CLASS}>
            <BrandLogo size={36} className="h-8 w-8 shrink-0 rounded-2xl object-cover sm:h-9 sm:w-9" />
            <span className="truncate">{t("brandName")}</span>
          </span>
        ) : (
          <Link href={`/${locale}`} className={BRAND_CLASS}>
            <BrandLogo size={36} className="h-8 w-8 shrink-0 rounded-2xl object-cover sm:h-9 sm:w-9" />
            <span className="truncate">{t("brandName")}</span>
          </Link>
        )}
        <div className="flex shrink-0 items-center gap-1.5">
          <LocaleSwitcher compact />
          <ThemeToggle />
        </div>
      </header>

      <main className="flex flex-1 items-center justify-center p-3 sm:p-4">
        <div className="w-full max-w-sm animate-fade-in space-y-5 rounded-2xl border border-border bg-card p-5 sm:space-y-6 sm:p-8">
          {/* Telegram's own Back is the way out inside the Mini App. */}
          {!inMiniApp && (
            <Link
              href={`/${locale}`}
              className="inline-flex min-h-11 items-center gap-1 text-xs font-semibold text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <ArrowLeft aria-hidden="true" className="h-3.5 w-3.5" /> {t("backHome")}
            </Link>
          )}

          <div className="space-y-2">
            <h1 className="font-display text-2xl font-extrabold tracking-tight">
              {t("title")}
            </h1>
            <p className="text-sm text-muted-foreground">
              {inMiniApp ? t("subtitle") : t("subtitleWithTelegram")}
            </p>
          </div>

          {/* Someone who was thrown out mid-session arrives here without having
              asked to; say why. Yields to a real submit error so the form never
              shows two banners at once. */}
          {sessionExpired && !error && (
            <div
              role="status"
              className="rounded-xl border border-accent/40 bg-accent/10 p-3 text-xs font-semibold text-foreground"
            >
              {t("sessionExpiredNotice")}
            </div>
          )}

          {/* Website only: inside Telegram the Mini App signs in with
              Telegram's own launch data and phone share instead. */}
          {!inMiniApp && (
            <div className="space-y-4">
              <TelegramLogin
                ref={telegramLogin}
                mode="login"
                onSuccess={(r) => finishAuth(r.mustChangePassword, true)}
              />
              <div className="flex items-center gap-3 text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
                <span aria-hidden="true" className="h-px flex-1 bg-border" />
                {tgLoginT("orDivider")}
                <span aria-hidden="true" className="h-px flex-1 bg-border" />
              </div>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
              <label
                htmlFor="login-phone"
                className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground"
              >
                {t("phoneLabel")}
              </label>
              <div className="flex items-center gap-2 rounded-2xl border border-border bg-background px-4 py-3 text-sm transition-colors focus-within:border-accent focus-within:ring-2 focus-within:ring-ring">
                <Phone aria-hidden="true" className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="font-bold text-foreground">+998</span>
                <input
                  id="login-phone"
                  type="tel"
                  inputMode="numeric"
                  autoComplete="tel-national"
                  value={formatNationalPhone(phone)}
                  onChange={(e) => {
                    setPhone(normalizePhone(e.target.value));
                    // The signed contact vouches for the number that was shared;
                    // an edited number is no longer that one.
                    setTgContact(null);
                  }}
                  placeholder="90 123 45 67"
                  className="w-full bg-transparent font-bold tracking-wide outline-none placeholder:font-normal placeholder:text-muted-foreground"
                  aria-label={t("phoneLabel")}
                />
              </div>
              {/* Part of the phone group: another way to fill the same field. */}
              <TelegramPhoneButton
                onPhone={(national, signed) => {
                  setPhone(national);
                  setTgContact(signed);
                }}
              />
            </div>

            <div className="space-y-1.5">
              <label
                htmlFor="login-password"
                className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground"
              >
                {t("passwordLabel")}
              </label>
              <div className="flex items-center gap-2 rounded-2xl border border-border bg-background px-4 py-3 text-sm transition-colors focus-within:border-accent focus-within:ring-2 focus-within:ring-ring">
                <Lock aria-hidden="true" className="h-4 w-4 shrink-0 text-muted-foreground" />
                <input
                  id="login-password"
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={t("passwordPlaceholder")}
                  className="w-full bg-transparent font-bold tracking-wide outline-none placeholder:font-normal placeholder:text-muted-foreground"
                  aria-label={t("passwordLabel")}
                />
              </div>
            </div>

            <div className="flex justify-end">
              <Link
                href={`/${locale}/forgot-password`}
                className="min-h-11 text-sm font-extrabold text-accent hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                {t("forgotPassword")}
              </Link>
            </div>

            {error === "password_not_set" ? (
              // An account made through Telegram has no password: say so and
              // offer both ways forward instead of a dead-end error.
              <div role="alert" className="space-y-3 rounded-xl border border-accent/40 bg-accent/10 p-3">
                <p className="text-sm font-extrabold text-foreground">{t("passwordlessTitle")}</p>
                <p className="text-xs font-semibold leading-snug text-foreground">{t("passwordlessBody")}</p>
                <div className="grid gap-2 sm:grid-cols-2">
                  <button
                    type="button"
                    onClick={() => {
                      if (inMiniApp) {
                        router.push(`/${locale}/tg`);
                        return;
                      }
                      setError(null);
                      telegramLogin.current?.start();
                    }}
                    className="flex min-h-11 items-center justify-center gap-2 rounded-xl bg-[#1f75bc] px-3 text-sm font-extrabold text-white hover:bg-[#1a66a5] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                  >
                    <Send aria-hidden="true" className="h-4 w-4" />
                    {t("passwordlessTelegram")}
                  </button>
                  <Link
                    href={`/${locale}/forgot-password${phone ? `?phone=${encodeURIComponent(phone)}` : ""}`}
                    className="flex min-h-11 items-center justify-center gap-2 rounded-xl border border-border bg-card px-3 text-sm font-extrabold text-foreground hover:border-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <KeyRound aria-hidden="true" className="h-4 w-4" />
                    {t("passwordlessSetPassword")}
                  </Link>
                </div>
              </div>
            ) : (
              error && (
                <div
                  role="alert"
                  className="rounded-xl border border-danger/50 bg-danger/10 p-3 text-xs font-semibold text-danger"
                >
                  {t(ERROR_MESSAGE_KEYS[error] ?? "errorUnknown")}
                </div>
              )
            )}

            {waitingForTelegram && (
              <p role="status" className="flex items-center gap-2 text-xs font-semibold text-muted-foreground">
                <Loader2 aria-hidden="true" className="h-3.5 w-3.5 animate-spin motion-reduce:animate-none" /> {tgT("connecting")}
              </p>
            )}

            <Button
              type="submit"
              variant="game"
              size="lg"
              className="w-full py-3 text-sm font-extrabold"
              disabled={submitting || waitingForTelegram}
            >
              {submitting ? t("submitting") : t("submit")}
            </Button>
          </form>

          <div className="space-y-3">
            <p className="text-center text-sm font-semibold text-muted-foreground">{t("noAccount")}</p>
            <Link href={`/${locale}/register${carryNext}`} className="block">
              <Button as="span" variant="outline" size="lg" className="w-full text-sm font-extrabold">
                {t("registerLink")}
              </Button>
            </Link>
          </div>

          <div className="flex items-center justify-center gap-1.5 text-[11px] text-muted-foreground">
            <ShieldCheck aria-hidden="true" className="h-3.5 w-3.5 text-success" />
            <span>{t("secureNote")}</span>
          </div>
        </div>
      </main>
    </div>
  );
}
