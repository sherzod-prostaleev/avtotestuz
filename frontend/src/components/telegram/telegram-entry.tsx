"use client";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import {
  Clock,
  Hourglass,
  LifeBuoy,
  Loader2,
  RotateCw,
  Send,
  ShieldAlert,
  ShieldCheck,
  Smartphone,
  WifiOff,
} from "lucide-react";
import { nationalPhoneFromShared } from "@/lib/phone-format";
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
import { BOT_USERNAME } from "@/lib/telegram/bot-username";
import { continueName } from "@/lib/telegram/continue-name";
import { forgetNeedPhone, recallNeedPhone, rememberNeedPhone } from "@/lib/telegram/need-phone-cache";
import { supportTelegramUrl } from "@/lib/site-contacts";

type Phase =
  // Probing the session / reading CloudStorage: nothing is signing in yet.
  | "loading"
  // The Telegram sign-in call itself is in flight.
  | "signing_in"
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
// The sign-in limiter counts per IP; retrying at once only burns another slot
// and lands on the same screen.
const RATE_LIMIT_COOLDOWN_S = 30;
// The public website, for the "bot unavailable" screen. A real link, so a
// learner stuck there has somewhere to go.
const WEBSITE_ORIGIN = "https://drivergo.uz";

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

/** Our support chat for a blocked account, from the public CMS contacts. */
async function loadSupportUrl(lifetime: AbortSignal): Promise<string> {
  try {
    return await withTimeout(lifetime, ME_PROBE_TIMEOUT_MS, async (signal) => {
      const res = await fetch("/api/proxy/site/contacts", { cache: "no-store", signal });
      if (!res.ok) return supportTelegramUrl(null);
      const json = (await res.json().catch(() => null)) as { data?: { telegramUrl?: string } } | null;
      return supportTelegramUrl(json?.data?.telegramUrl);
    });
  } catch {
    return supportTelegramUrl(null);
  }
}

/**
 * The invite code of a t.me/<bot>?startapp=ref_<CODE> launch, for the
 * password registration link (the one-tap phone path gets it server-side,
 * from the signed launch data). Same charset Telegram allows.
 */
export function referralFromStartParam(param: string | undefined): string | null {
  const m = /^ref_([A-Za-z0-9_-]{1,60})$/.exec(param ?? "");
  return m ? m[1] : null;
}

type PhoneSignInBody = {
  data?: { must_change_password?: boolean };
  error?: { code?: string };
};

/** `?next=…` for the welcome's links, only for a path /tg itself would honour. */
function nextQueryFrom(search: string, locale: string): string {
  const next = new URLSearchParams(search).get("next");
  if (next === null || safeNextPath(next, locale) === `/${locale}/dashboard`) return "";
  return `?next=${encodeURIComponent(next)}`;
}

/**
 * What a learner sees the moment the Mini App opens from the bot. A live
 * cookie session goes straight in; otherwise Telegram's signed launch data
 * signs a linked account in silently, and an unlinked one gets the ordinary
 * phone login/register. Every failure ends on a screen that says what to do.
 *
 * `botUsername` (server env, already validated) lets a plain-browser visitor
 * open the bot instead of hitting a dead end.
 */
export function TelegramEntry({ botUsername = null }: { botUsername?: string | null } = {}) {
  const t = useTranslations("TelegramApp");
  const loginT = useTranslations("Login");
  const locale = useLocale() as Locale;
  const router = useRouter();
  const webApp = useTelegram();
  const [phase, setPhase] = useState<Phase>("loading");
  const [firstName, setFirstName] = useState("");
  const [canContinue, setCanContinue] = useState(false);
  // Read once on mount: the welcome only renders after effects, so this never
  // differs between the server and the first client render.
  const [nextQuery, setNextQuery] = useState("");
  const [supportUrl, setSupportUrl] = useState<string | null>(null);
  // A one-line problem with the phone share, shown on the welcome screen.
  const [phoneNotice, setPhoneNotice] = useState<string | null>(null);
  const [sharingPhone, setSharingPhone] = useState(false);
  // Seconds left before a rate-limited retry is offered. One interval
  // against a deadline, so a throttled timer (background tab) cannot stretch it.
  const [cooldown, setCooldown] = useState(0);
  const [retryAt, setRetryAt] = useState<number | null>(null);
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

  // After any Telegram sign-in set the cookies: prove they stuck (Safari on
  // web.telegram.org refuses third-party cookies even when partitioned),
  // forget the need-phone verdict and the sign-out flag, then go in.
  const enterSignedIn = useCallback(
    async (lifetime: AbortSignal, mustChangePassword: boolean) => {
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
      forgetNeedPhone();
      // cloudRemove has a 3s timeout, so await is safe: it will never hang.
      await cloudRemove(AUTOLOGIN_OFF_KEY);
      if (lifetime.aborted) return;
      goIn(mustChangePassword || me.mustChangePassword);
    },
    [goIn]
  );

  const signIn = useCallback(
    async (app: TelegramWebApp, lifetime: AbortSignal) => {
      setPhase("signing_in");
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
        const next = phaseForError(reply.status, json?.error?.code);
        if (next === "rate_limited") {
          setCooldown(RATE_LIMIT_COOLDOWN_S);
          setRetryAt(Date.now() + RATE_LIMIT_COOLDOWN_S * 1000);
        }
        if (next === "blocked") void loadSupportUrl(lifetime).then((url) => !lifetime.aborted && setSupportUrl(url));
        setPhase(next);
        return;
      }
      if (json?.data?.need_phone) {
        const name = json.data.first_name || app.initDataUnsafe.user?.first_name || "";
        rememberNeedPhone(app.initDataUnsafe.user?.id, name);
        await showWelcome(lifetime, name, false);
        return;
      }
      if (!json?.data) {
        setPhase("error");
        return;
      }
      await enterSignedIn(lifetime, json.data.must_change_password === true);
    },
    [enterSignedIn, showWelcome]
  );

  // One tap: Telegram's own "share your number" sheet, then the server finds
  // the learner with that +998 number — or creates one, no password — links
  // this Telegram account and signs in. Login/register stay for passwords.
  const signInWithPhone = useCallback(() => {
    const lifetime = lifetimeRef.current?.signal;
    const app = webApp;
    if (!app || !lifetime || busy.current || typeof app.requestContact !== "function") return;
    busy.current = true;
    setPhoneNotice(null);
    setSharingPhone(true);
    const release = () => {
      if (!lifetime.aborted) {
        busy.current = false;
        setSharingPhone(false);
      }
    };
    try {
      app.requestContact((shared, res) => {
        if (lifetime.aborted) return;
        const contact = typeof res?.response === "string" ? res.response : "";
        if (!shared || !contact) {
          setPhoneNotice(t("phoneDeclined"));
          release();
          return;
        }
        const raw = res?.responseUnsafe?.contact?.phone_number;
        if (raw && nationalPhoneFromShared(raw) === null) {
          setPhoneNotice(t("phoneNotUzbek"));
          release();
          return;
        }
        void (async () => {
          try {
            setPhase("signing_in");
            let reply: { ok: boolean; status: number; json: PhoneSignInBody | null };
            try {
              reply = await withTimeout(lifetime, SIGN_IN_TIMEOUT_MS, async (signal) => {
                const r = await fetch("/api/auth/telegram/phone", {
                  method: "POST",
                  headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ init_data: app.initData, contact }),
                  signal,
                });
                return { ok: r.ok, status: r.status, json: (await r.json().catch(() => null)) as PhoneSignInBody | null };
              });
            } catch {
              if (!lifetime.aborted) setPhase("error");
              return;
            }
            if (lifetime.aborted) return;
            if (!reply.ok) {
              if (reply.json?.error?.code === "invalid_phone") {
                setPhoneNotice(t("phoneNotUzbek"));
                setPhase("welcome");
                return;
              }
              const next = phaseForError(reply.status, reply.json?.error?.code);
              if (next === "rate_limited") {
                setCooldown(RATE_LIMIT_COOLDOWN_S);
                setRetryAt(Date.now() + RATE_LIMIT_COOLDOWN_S * 1000);
              }
              if (next === "blocked") void loadSupportUrl(lifetime).then((url) => !lifetime.aborted && setSupportUrl(url));
              setPhase(next);
              return;
            }
            await enterSignedIn(lifetime, reply.json?.data?.must_change_password === true);
          } finally {
            release();
          }
        })();
      });
    } catch {
      // Old client without the sheet after all: the password paths remain.
      release();
    }
  }, [enterSignedIn, t, webApp]);

  const enter = useCallback(
    async (app: TelegramWebApp, lifetime: AbortSignal) => {
      if (busy.current) return;
      busy.current = true;
      // Each attempt re-probes the session: a verdict from a failed earlier
      // attempt (whose logout may have half-worked) must not trigger a logout
      // for whatever session, if any, is there now.
      strangerSession.current = false;
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
          // Back from /login remounts this page: the server already said this
          // Telegram account needs a phone sign-in, and nothing since could
          // have changed that (a successful sign-in clears the verdict).
          const known = recallNeedPhone(app.initDataUnsafe.user?.id);
          if (known) {
            await showWelcome(lifetime, known.firstName, false);
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
    setNextQuery(nextQueryFrom(window.location.search, locale));
  }, [locale]);

  useEffect(() => {
    if (retryAt === null) return;
    const timer = window.setInterval(() => {
      const left = Math.max(0, Math.ceil((retryAt - Date.now()) / 1000));
      setCooldown(left);
      if (left === 0) setRetryAt(null);
    }, 1000);
    return () => window.clearInterval(timer);
  }, [retryAt]);

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
  // whatever the spinner left behind. Focus alone announces it; the notices
  // carry no role="alert", which would read the same screen out twice.
  useEffect(() => {
    if (phase !== "loading" && phase !== "signing_in") headingRef.current?.focus();
  }, [phase]);

  // Inside Telegram the learner can always close back to the chat.
  const backToBot = webApp ? <BackToBotButton label={t("backToBot")} onClick={() => webApp.close()} /> : undefined;

  // A plain browser has no chat to close back to: offer the bot itself (only
  // our configured one, re-checked here) and the ordinary website login.
  const safeBot = botUsername && BOT_USERNAME.test(botUsername) ? botUsername : null;
  const outsideActions = webApp ? (
    backToBot
  ) : (
    <div className="space-y-3">
      {safeBot && (
        <a
          href={`https://t.me/${safeBot}`}
          className="block rounded-2xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <Button as="span" variant="game" size="lg" className="w-full text-sm font-extrabold">
            <Send aria-hidden="true" className="mr-2 h-4 w-4" />
            {t("openBot")}
          </Button>
        </a>
      )}
      <Link
        href={`/${locale}/login`}
        className="block rounded-2xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <Button as="span" variant={safeBot ? "outline" : "game"} size="lg" className="w-full text-sm font-extrabold">
          {t("loginOnSite")}
        </Button>
      </Link>
    </div>
  );

  const websiteUrl = `${WEBSITE_ORIGIN}/${locale}`;
  const unavailableBody = t.rich("unavailableBody", {
    site: (chunks) => (
      <a
        href={websiteUrl}
        target="_blank"
        rel="noopener noreferrer"
        className="font-semibold text-accent-ink underline underline-offset-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        onClick={(event) => {
          // Inside Telegram the site opens in its browser, not over the app.
          if (!webApp) return;
          try {
            webApp.openLink(websiteUrl);
            event.preventDefault();
          } catch {
            /* old client: the plain link still works */
          }
        }}
      >
        {chunks}
      </a>
    ),
  });

  const continueLabel = (() => {
    const short = continueName(firstName);
    return short ? t("continueAs", { name: short }) : t("continueFallback");
  })();

  // The phone share needs Bot API 6.9+ (requestContact); a linked learner
  // offered "continue as" does not need it.
  const canPhoneSignIn = !canContinue && typeof webApp?.requestContact === "function";
  // Password registration keeps a startapp=ref_<CODE> invite: /register
  // stores ?ref= and applies it to the new account.
  const referral = referralFromStartParam(webApp?.initDataUnsafe.start_param);
  const registerQuery = referral
    ? `${nextQuery ? `${nextQuery}&` : "?"}ref=${encodeURIComponent(referral)}`
    : nextQuery;

  const retry = () => {
    const lifetime = lifetimeRef.current?.signal;
    if (webApp && lifetime) void enter(webApp, lifetime);
  };

  return (
    <div
      className="asphalt-hero auth-safe-top auth-safe-bottom flex min-h-[100dvh] flex-col bg-background"
    >
      {/* Top-aligned, not centred: loading → welcome grows the card, and a
          centred card would jump up by half the difference. */}
      <main className="flex flex-1 items-start justify-center px-4 pb-4 pt-[clamp(1rem,10dvh,6rem)]">
        <div className="w-full max-w-sm animate-fade-in space-y-6 rounded-2xl border border-border bg-card p-5 text-center sm:p-8">
          <div className="flex flex-col items-center gap-2">
            <BrandLogo size={64} priority className="h-16 w-16 rounded-3xl object-cover" />
            <span className="font-display text-lg font-black text-foreground">{loginT("brandName")}</span>
          </div>

          {(phase === "loading" || phase === "signing_in") && (
            <p
              role="status"
              className="flex min-h-12 flex-col items-center justify-center gap-2 text-sm font-semibold text-muted-foreground"
            >
              <Loader2 aria-hidden="true" className="h-5 w-5 animate-spin motion-reduce:animate-none" />
              {phase === "signing_in" ? t("loading") : t("connecting")}
            </p>
          )}

          {phase === "welcome" && (
            <div className="space-y-5">
              <div className="space-y-2">
                <h1
                  ref={headingRef}
                  tabIndex={-1}
                  // A long single-word Telegram name must wrap, not scroll the page.
                  className="font-display text-2xl font-extrabold tracking-tight outline-none [overflow-wrap:anywhere]"
                >
                  {firstName ? t("welcomeNamed", { name: firstName }) : t("welcome")}
                </h1>
                <p className="text-sm text-muted-foreground">{t("welcomeHint")}</p>
                {/* need_phone also answers an account linked before phone
                    verification existed: say once why a phone is asked for. */}
                {!canContinue && (
                  <p className="flex items-start gap-2 rounded-xl border border-border bg-background/60 px-3 py-2.5 text-left text-xs font-semibold leading-snug text-foreground">
                    <ShieldCheck aria-hidden="true" className="mt-px h-4 w-4 shrink-0 text-success" />
                    <span>{t("phoneConfirmNote")}</span>
                  </p>
                )}
              </div>
              <div className="space-y-3">
                {canPhoneSignIn && (
                  <div className="space-y-2">
                    <Button
                      variant="game"
                      size="lg"
                      className="!h-auto min-h-12 w-full !px-4 py-3 text-sm font-extrabold"
                      disabled={sharingPhone}
                      aria-busy={sharingPhone}
                      onClick={signInWithPhone}
                    >
                      <span className="min-w-0 break-words">{t("continueWithPhone")}</span>
                    </Button>
                    <p className="text-xs font-semibold leading-snug text-muted-foreground">{t("continueWithPhoneHint")}</p>
                    {phoneNotice && (
                      <p role="alert" className="text-xs font-semibold text-danger">
                        {phoneNotice}
                      </p>
                    )}
                    <div className="flex items-center gap-3 pt-1 text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
                      <span aria-hidden="true" className="h-px flex-1 bg-border" />
                      {t("orPassword")}
                      <span aria-hidden="true" className="h-px flex-1 bg-border" />
                    </div>
                  </div>
                )}
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
                    <span className="min-w-0 break-words">{continueLabel}</span>
                  </Button>
                )}
                <Link href={`/${locale}/login${nextQuery}`} className="block rounded-2xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                  <Button as="span" variant={canContinue || canPhoneSignIn ? "outline" : "game"} size="lg" className="w-full text-sm font-extrabold">
                    {t("login")}
                  </Button>
                </Link>
                <Link href={`/${locale}/register${registerQuery}`} className="block rounded-2xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                  <Button as="span" variant="outline" size="lg" className="w-full text-sm font-extrabold">
                    {t("register")}
                  </Button>
                </Link>
              </div>
            </div>
          )}

          {phase === "outside" && (
            <Notice
              headingRef={headingRef}
              icon={<Send className="h-6 w-6" />}
              title={t("outsideTitle")}
              body={t("outsideBody")}
              action={outsideActions}
            />
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
              body={unavailableBody}
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
              action={
                <div className="space-y-3">
                  {supportUrl && (
                    <a
                      href={supportUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="block rounded-2xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    >
                      <Button
                        as="span"
                        variant={webApp ? "outline" : "game"}
                        size="lg"
                        className="!h-auto min-h-12 w-full py-3 text-sm font-extrabold"
                      >
                        <LifeBuoy aria-hidden="true" className="mr-2 h-4 w-4" />
                        {t("contactSupport")}
                      </Button>
                    </a>
                  )}
                  {backToBot}
                </div>
              }
            />
          )}
          {phase === "rate_limited" && (
            <Notice
              headingRef={headingRef}
              icon={<Hourglass className="h-6 w-6" />}
              title={t("rateLimitedTitle")}
              body={t("rateLimitedBody")}
              action={
                <RetryButton
                  label={cooldown > 0 ? t("retryIn", { seconds: cooldown }) : t("retry")}
                  disabled={cooldown > 0}
                  onClick={retry}
                />
              }
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

function RetryButton({ label, onClick, disabled = false }: { label: string; onClick: () => void; disabled?: boolean }) {
  return (
    <Button variant="game" size="lg" className="w-full text-sm font-extrabold tabular-nums" disabled={disabled} onClick={onClick}>
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
  body: ReactNode;
  action?: ReactNode;
  tone?: "accent" | "danger";
}) {
  return (
    <div className="space-y-5">
      <div className="space-y-3">
        {/* accent-ink, not accent: the CTA amber on its own 10% tint is
            ~2:1 in the light theme, under the 3:1 a meaningful icon needs. */}
        <div
          aria-hidden="true"
          className={`mx-auto flex h-12 w-12 items-center justify-center rounded-2xl ${
            tone === "danger" ? "bg-danger/10 text-danger" : "bg-accent/10 text-accent-ink"
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
