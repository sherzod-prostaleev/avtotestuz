"use client";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { Clock, Hourglass, Loader2, RotateCw, Send, ShieldAlert, Smartphone, WifiOff } from "lucide-react";
import { BrandLogo } from "@/components/brand/brand-logo";
import { Button } from "@/components/ui/button";
import { useTelegram } from "@/components/telegram/telegram-provider";
import type { Locale } from "@/i18n/config";
import {
  AUTOLOGIN_OFF_KEY,
  cloudGetResult,
  cloudRemove,
  isTelegramMiniApp,
  markTelegramMiniApp,
  type TelegramWebApp,
} from "@/lib/telegram/web-app";
import { resolveTelegramLocale } from "@/lib/telegram/locale";
import { safeNextPath } from "@/lib/telegram/safe-next";

type Phase =
  | "loading"
  | "welcome"
  | "outside"
  | "cookie_blocked"
  | "unavailable"
  | "rate_limited"
  | "blocked"
  | "error"
  | "sdk_failed";

// The SDK script normally loads in well under a second; past this, with no
// launch data in the URL, the page was opened as a plain link, not by Telegram.
const SDK_WAIT_MS = 3000;
// With launch data present it *is* Telegram, so give a slow mobile network
// longer before calling the SDK lost.
const SDK_LAUNCH_WAIT_MS = 10000;

// A stalled mobile connection must end on the retry screen, never on an
// endless spinner.
const SIGN_IN_TIMEOUT_MS = 15000;
// The probe only decides "already signed in?" and its failure falls through to
// sign-in anyway, so it must not hold the spinner for the full sign-in budget.
const ME_PROBE_TIMEOUT_MS = 8000;

/**
 * Runs `run` with a signal that aborts after `timeoutMs` (fake-timer friendly,
 * unlike AbortSignal.timeout) or as soon as `lifetime` aborts, i.e. the page
 * unmounted. The timer is always cleared so nothing outlives the component.
 */
async function withTimeout<T>(
  lifetime: AbortSignal,
  timeoutMs: number,
  run: (signal: AbortSignal) => Promise<T>
): Promise<T> {
  const controller = new AbortController();
  const abort = () => controller.abort();
  if (lifetime.aborted) abort();
  else lifetime.addEventListener("abort", abort, { once: true });
  const timer = setTimeout(abort, timeoutMs);
  try {
    return await run(controller.signal);
  } finally {
    clearTimeout(timer);
    lifetime.removeEventListener("abort", abort);
  }
}

type MeProbe = { kind: "ok"; mustChangePassword: boolean } | { kind: "unauthorized" } | { kind: "failed" };

async function probeMe(lifetime: AbortSignal): Promise<MeProbe> {
  try {
    return await withTimeout(lifetime, ME_PROBE_TIMEOUT_MS, async (signal): Promise<MeProbe> => {
      const res = await fetch("/api/proxy/me", { cache: "no-store", signal });
      if (res.status === 401) return { kind: "unauthorized" };
      if (!res.ok) return { kind: "failed" };
      const json = (await res.json().catch(() => null)) as
        | { data?: { profile?: { must_change_password?: boolean } } }
        | null;
      return { kind: "ok", mustChangePassword: json?.data?.profile?.must_change_password === true };
    });
  } catch {
    return { kind: "failed" };
  }
}

/**
 * The Telegram user id the signed-in profile is linked to, or null when it
 * is not linked or we cannot tell (failure, timeout, older API without the
 * field). Only a definite other id changes what /tg does.
 */
async function probeLinkedTelegramId(lifetime: AbortSignal): Promise<number | null> {
  try {
    return await withTimeout(lifetime, ME_PROBE_TIMEOUT_MS, async (signal) => {
      const res = await fetch("/api/proxy/me/telegram", { cache: "no-store", signal });
      if (!res.ok) return null;
      const json = (await res.json().catch(() => null)) as
        | { data?: { linked?: boolean; tg_user_id?: unknown } }
        | null;
      const id = json?.data?.linked === true ? json.data.tg_user_id : null;
      return typeof id === "number" && id > 0 ? id : null;
    });
  } catch {
    return null;
  }
}

/**
 * Ends the cookie session (tgp-mode cookies included). False when it could not
 * be confirmed, so the caller never shows a welcome whose Kirish link would
 * bounce into the stranger's account.
 */
async function dropSession(lifetime: AbortSignal): Promise<boolean> {
  try {
    return await withTimeout(lifetime, ME_PROBE_TIMEOUT_MS, async (signal) => {
      const res = await fetch("/api/auth/logout", { method: "POST", signal });
      return res.ok;
    });
  } catch {
    return false;
  }
}

type SignInBody = {
  data?: { need_phone?: boolean; first_name?: string; must_change_password?: boolean };
  error?: { code?: string };
};

function phaseForError(status: number, code: string | undefined): Phase {
  switch (code) {
    case "invalid_init_data":
      return "outside";
    case "telegram_bot_unconfigured":
      return "unavailable";
    case "rate_limited":
      return "rate_limited";
    case "account_blocked":
      return "blocked";
  }
  if (status === 429) return "rate_limited";
  // network_error, cross_site, 5xx and anything unforeseen: retryable.
  return "error";
}

/**
 * What a learner sees the moment the Mini App opens from the bot. A live
 * cookie session goes straight in; otherwise Telegram's signed launch data
 * signs a linked account in silently, and an unlinked one gets the ordinary
 * phone login/register. Every failure ends on a screen that says what to do.
 */
export function TelegramEntry() {
  const t = useTranslations("TelegramApp");
  const loginT = useTranslations("Login");
  const locale = useLocale() as Locale;
  const router = useRouter();
  const webApp = useTelegram();
  const [phase, setPhase] = useState<Phase>("loading");
  const [firstName, setFirstName] = useState("");
  const [canContinue, setCanContinue] = useState(false);
  const started = useRef(false);
  const busy = useRef(false);
  // Set when a cookie session linked to a DIFFERENT Telegram account is still
  // present. It must not outlive a welcome screen: /login and /register
  // redirect signed-in visitors to the dashboard, i.e. into that account.
  const strangerSession = useRef(false);
  // A retry after "continue as" must not bounce the learner back to welcome.
  const explicitSignIn = useRef(false);
  const headingRef = useRef<HTMLHeadingElement>(null);
  // Aborted on unmount so in-flight fetches stop and nothing navigates or sets
  // state afterwards. Created in an effect, not in render: StrictMode runs
  // mount, unmount, mount, and the second mount needs a live controller.
  const lifetimeRef = useRef<AbortController | null>(null);

  const goIn = useCallback(
    (mustChangePassword: boolean) => {
      if (mustChangePassword) {
        router.replace(`/${locale}/change-password`);
        return;
      }
      // Read off location instead of useSearchParams, which would opt the
      // page into a client-render bailout at build time (as on /login).
      const next = new URLSearchParams(window.location.search).get("next");
      router.replace(safeNextPath(next, locale));
    },
    [locale, router]
  );

  // Shows the welcome screen; first ends a stranger's session if there is one.
  // Not a deliberate sign-out, so autologin_off is left alone.
  const showWelcome = useCallback(async (lifetime: AbortSignal, name: string, continuable: boolean) => {
    if (strangerSession.current) {
      setPhase("loading");
      const dropped = await dropSession(lifetime);
      if (lifetime.aborted) return;
      if (!dropped) {
        setPhase("error");
        return;
      }
      strangerSession.current = false;
    }
    setFirstName(name);
    setCanContinue(continuable);
    setPhase("welcome");
  }, []);

  const signIn = useCallback(
    async (app: TelegramWebApp, lifetime: AbortSignal) => {
      setPhase("loading");
      let reply: { ok: boolean; status: number; json: SignInBody | null };
      try {
        reply = await withTimeout(lifetime, SIGN_IN_TIMEOUT_MS, async (signal) => {
          const res = await fetch("/api/auth/telegram", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ init_data: app.initData }),
            signal,
          });
          return { ok: res.ok, status: res.status, json: (await res.json().catch(() => null)) as SignInBody | null };
        });
      } catch {
        if (!lifetime.aborted) setPhase("error");
        return;
      }
      if (lifetime.aborted) return;
      const { json } = reply;
      if (!reply.ok) {
        setPhase(phaseForError(reply.status, json?.error?.code));
        return;
      }
      if (json?.data?.need_phone) {
        await showWelcome(lifetime, json.data.first_name || app.initDataUnsafe.user?.first_name || "", false);
        return;
      }
      if (!json?.data) {
        setPhase("error");
        return;
      }
      // Safari on web.telegram.org refuses third-party cookies even when
      // partitioned; prove the cookie stuck before handing over to the app.
      const me = await probeMe(lifetime);
      if (lifetime.aborted) return;
      if (me.kind === "unauthorized") {
        setPhase("cookie_blocked");
        return;
      }
      if (me.kind === "failed") {
        setPhase("error");
        return;
      }
      // cloudRemove has a 3s timeout, so await is safe: it will never hang.
      await cloudRemove(AUTOLOGIN_OFF_KEY);
      if (lifetime.aborted) return;
      goIn(json.data.must_change_password === true || me.mustChangePassword);
    },
    [goIn, showWelcome]
  );

  const enter = useCallback(
    async (app: TelegramWebApp, lifetime: AbortSignal) => {
      if (busy.current) return;
      busy.current = true;
      try {
        setPhase("loading");
        // A live session means they never signed out: go straight in, with no
        // sign-in call (saves a rate-limit slot on shared carrier IPs) and
        // regardless of autologin_off.
        const me = await probeMe(lifetime);
        if (lifetime.aborted) return;
        if (me.kind === "ok") {
          // A shared phone or webview can hold someone else's session. If
          // that profile is linked to a DIFFERENT Telegram account than the
          // one launching us, the launching identity wins: sign in with it
          // below (or offer phone sign-in) instead of opening a stranger's
          // account. Unlinked or unknown keeps the fast path.
          const linkedId = await probeLinkedTelegramId(lifetime);
          if (lifetime.aborted) return;
          const launchingId = app.initDataUnsafe.user?.id;
          if (linkedId === null || launchingId === undefined || linkedId === launchingId) {
            goIn(me.mustChangePassword);
            return;
          }
          strangerSession.current = true;
        }
        // "failed" falls through: the sign-in call reports the real error.
        if (!explicitSignIn.current) {
          const flag = await cloudGetResult(AUTOLOGIN_OFF_KEY);
          if (lifetime.aborted) return;
          // A flag that could not be read is unknown, not absent: signing in
          // anyway would silently undo a deliberate sign-out (D6), so offer
          // the choice instead.
          if (flag.status === "unavailable" || flag.value === "1") {
            await showWelcome(lifetime, app.initDataUnsafe.user?.first_name || "", true);
            return;
          }
        }
        await signIn(app, lifetime);
      } finally {
        // An aborted run was already reset by the unmount cleanup; a newer
        // run (StrictMode's second mount) may own the flag by now.
        if (!lifetime.aborted) busy.current = false;
      }
    },
    [goIn, signIn, showWelcome]
  );

  const continueAsTelegramUser = useCallback(async () => {
    const lifetime = lifetimeRef.current?.signal;
    if (!webApp || !lifetime || busy.current) return;
    explicitSignIn.current = true;
    busy.current = true;
    try {
      await signIn(webApp, lifetime);
    } finally {
      if (!lifetime.aborted) busy.current = false;
    }
  }, [signIn, webApp]);

  useEffect(() => {
    const controller = new AbortController();
    lifetimeRef.current = controller;
    return () => {
      controller.abort();
      lifetimeRef.current = null;
      // Let a StrictMode remount start over instead of finding the first
      // run's flags still set.
      started.current = false;
      busy.current = false;
    };
  }, []);

  // Only a Telegram client may flag the tab as Telegram. A #tgWebAppData hash
  // proves nothing (anyone can plant their own in a link), and webApp is only
  // non-null when a real Telegram host is around (getWebApp).
  useEffect(() => {
    if (webApp) markTelegramMiniApp();
  }, [webApp]);

  useEffect(() => {
    if (webApp) return;
    const launched = isTelegramMiniApp();
    const timer = window.setTimeout(
      () => setPhase((p) => (p === "loading" ? (launched ? "sdk_failed" : "outside") : p)),
      launched ? SDK_LAUNCH_WAIT_MS : SDK_WAIT_MS
    );
    return () => window.clearTimeout(timer);
  }, [webApp]);

  useEffect(() => {
    const lifetime = lifetimeRef.current?.signal;
    if (!webApp || !lifetime || started.current) return;
    started.current = true;
    const target = resolveTelegramLocale(locale, webApp.initDataUnsafe.user?.language_code);
    if (target !== locale) {
      // replace, never a Link/prefetch: fetching another locale's URL in the
      // background rewrites the saved NEXT_LOCALE.
      // A `next` for this locale is rewritten to the target one so it still
      // passes safeNextPath there; anything else is dropped.
      const params = new URLSearchParams(window.location.search);
      const next = params.get("next");
      if (next !== null) {
        if (next.startsWith(`/${locale}/`)) params.set("next", `/${target}/${next.slice(locale.length + 2)}`);
        else params.delete("next");
      }
      const query = params.toString();
      router.replace(`/${target}/tg${query ? `?${query}` : ""}`);
      return;
    }
    void enter(webApp, lifetime);
  }, [enter, locale, router, webApp]);

  // Screen readers and keyboards land on the new state's heading, not on
  // whatever the spinner left behind.
  useEffect(() => {
    if (phase !== "loading") headingRef.current?.focus();
  }, [phase]);

  // Inside Telegram the learner can always close back to the chat; in a plain
  // browser there is no bot to return to, so those states stay text-only.
  const backToBot = webApp ? <BackToBotButton label={t("backToBot")} onClick={() => webApp.close()} /> : undefined;

  const retry = () => {
    const lifetime = lifetimeRef.current?.signal;
    if (webApp && lifetime) void enter(webApp, lifetime);
  };

  return (
    <div
      className="asphalt-hero auth-safe-top auth-safe-bottom flex min-h-[100dvh] flex-col bg-background"
    >
      <main className="flex flex-1 items-center justify-center p-4">
        <div className="w-full max-w-sm animate-fade-in space-y-6 rounded-2xl border border-border bg-card p-5 text-center sm:p-8">
          <div className="flex flex-col items-center gap-2">
            <BrandLogo size={64} priority className="h-16 w-16 rounded-3xl object-cover" />
            <span className="font-display text-lg font-black text-foreground">{loginT("brandName")}</span>
          </div>

          {phase === "loading" && (
            <p
              role="status"
              className="flex min-h-12 items-center justify-center gap-2 text-sm font-semibold text-muted-foreground"
            >
              <Loader2 aria-hidden="true" className="h-5 w-5 animate-spin motion-reduce:animate-none" />
              {t("loading")}
            </p>
          )}

          {phase === "welcome" && (
            <div className="space-y-5">
              <div className="space-y-2">
                <h1
                  ref={headingRef}
                  tabIndex={-1}
                  className="font-display text-2xl font-extrabold tracking-tight outline-none"
                >
                  {firstName ? t("welcomeNamed", { name: firstName }) : t("welcome")}
                </h1>
                <p className="text-sm text-muted-foreground">{t("welcomeHint")}</p>
              </div>
              <div className="space-y-3">
                {canContinue && (
                  <Button
                    variant="game"
                    size="lg"
                    // Wraps instead of truncating: a long Telegram name cut to
                    // "Abdurahmonbek …" hides what the button does.
                    className="!h-auto w-full !px-4 py-3 text-sm font-extrabold"
                    onClick={() => void continueAsTelegramUser()}
                  >
                    <Send aria-hidden="true" className="mr-2 h-4 w-4 shrink-0" />
                    <span className="min-w-0 break-words">
                      {firstName ? t("continueAs", { name: firstName }) : t("continueAs", { name: "Telegram" })}
                    </span>
                  </Button>
                )}
                <Link href={`/${locale}/login`} className="block rounded-2xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                  <Button as="span" variant={canContinue ? "outline" : "game"} size="lg" className="w-full text-sm font-extrabold">
                    {t("login")}
                  </Button>
                </Link>
                <Link href={`/${locale}/register`} className="block rounded-2xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                  <Button as="span" variant="outline" size="lg" className="w-full text-sm font-extrabold">
                    {t("register")}
                  </Button>
                </Link>
              </div>
            </div>
          )}

          {phase === "outside" && (
            <Notice headingRef={headingRef} icon={<Send className="h-6 w-6" />} title={t("outsideTitle")} body={t("outsideBody")} action={backToBot} />
          )}
          {phase === "cookie_blocked" && (
            <Notice
              headingRef={headingRef}
              icon={<Smartphone className="h-6 w-6" />}
              title={t("cookieTitle")}
              body={t("cookieBody")}
              action={backToBot}
            />
          )}
          {phase === "unavailable" && (
            <Notice
              headingRef={headingRef}
              icon={<Clock className="h-6 w-6" />}
              title={t("unavailableTitle")}
              body={t("unavailableBody")}
              action={backToBot}
            />
          )}
          {phase === "blocked" && (
            <Notice
              headingRef={headingRef}
              tone="danger"
              icon={<ShieldAlert className="h-6 w-6" />}
              title={t("blockedTitle")}
              body={loginT("errorAccountBlocked")}
              action={backToBot}
            />
          )}
          {phase === "rate_limited" && (
            <Notice
              headingRef={headingRef}
              icon={<Hourglass className="h-6 w-6" />}
              title={t("rateLimitedTitle")}
              body={t("rateLimitedBody")}
              action={<RetryButton label={t("retry")} onClick={retry} />}
            />
          )}
          {phase === "error" && (
            <Notice
              headingRef={headingRef}
              tone="danger"
              icon={<WifiOff className="h-6 w-6" />}
              title={t("errorTitle")}
              body={t("errorBody")}
              action={<RetryButton label={t("retry")} onClick={retry} />}
            />
          )}
          {phase === "sdk_failed" && (
            <Notice
              headingRef={headingRef}
              tone="danger"
              icon={<WifiOff className="h-6 w-6" />}
              title={t("errorTitle")}
              body={t("errorBody")}
              // Nothing to call without the SDK: reloading re-fetches it.
              action={<RetryButton label={t("retry")} onClick={() => window.location.reload()} />}
            />
          )}
        </div>
      </main>
    </div>
  );
}

function BackToBotButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <Button variant="game" size="lg" className="w-full text-sm font-extrabold" onClick={onClick}>
      <Send aria-hidden="true" className="mr-2 h-4 w-4" />
      {label}
    </Button>
  );
}

function RetryButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <Button variant="game" size="lg" className="w-full text-sm font-extrabold" onClick={onClick}>
      <RotateCw aria-hidden="true" className="mr-2 h-4 w-4" />
      {label}
    </Button>
  );
}

function Notice({
  headingRef,
  icon,
  title,
  body,
  action,
  tone = "accent",
}: {
  headingRef: React.RefObject<HTMLHeadingElement | null>;
  icon: ReactNode;
  title: string;
  body: string;
  action?: ReactNode;
  tone?: "accent" | "danger";
}) {
  return (
    <div className="space-y-5">
      <div role="alert" className="space-y-3">
        <div
          aria-hidden="true"
          className={`mx-auto flex h-12 w-12 items-center justify-center rounded-2xl ${
            tone === "danger" ? "bg-danger/10 text-danger" : "bg-accent/10 text-accent"
          }`}
        >
          {icon}
        </div>
        <h1 ref={headingRef} tabIndex={-1} className="font-display text-xl font-extrabold tracking-tight outline-none">
          {title}
        </h1>
        <p className="text-sm text-muted-foreground">{body}</p>
      </div>
      {action}
    </div>
  );
}
