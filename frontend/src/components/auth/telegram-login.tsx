"use client";

import { useCallback, useEffect, useImperativeHandle, useRef, useState, type Ref } from "react";
import { useTranslations } from "next-intl";
import { ExternalLink, Loader2, RotateCw, Send, ShieldAlert, TimerOff, XCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { QrCode } from "@/components/auth/qr-code";
import { rememberSignedInAs } from "@/lib/signed-in-notice";
import { fetchTelegramLoginEnabled } from "@/lib/telegram-login-flag";

/**
 * Website «Telegram orqali kirish» (also used on /register: the same request
 * signs in an existing learner or creates one). Start → open the bot → poll
 * until the learner approves in Telegram → complete → the caller's onSuccess.
 *
 * The pending request survives a full navigation (mobile browsers may leave
 * this page for t.me when Telegram does not intercept the link): it is kept in
 * sessionStorage and resumed on mount. The cookie half of it is HttpOnly.
 */

export type TelegramLoginResult = { mustChangePassword: boolean; created: boolean };
export type TelegramLoginHandle = { start: () => void };

type Phase = "idle" | "starting" | "waiting" | "completing" | "cancelled" | "blocked" | "expired" | "error";

type Pending = { token: string; botURL: string; expiresAt: number };

export const TELEGRAM_LOGIN_STORAGE_KEY = "drivergo:tglogin";
const POLL_MS = 2000;
const MAX_POLL_BACKOFF_MS = 10000;
// An approval in the last seconds can still complete (backend grace: 2 min).
const COMPLETE_GRACE_MS = 60_000;

const START_ERRORS: Record<string, string> = {
  telegram_bot_unconfigured: "errorUnavailable",
  // The telegram_login kill switch (admin → flags).
  telegram_login_disabled: "errorUnavailable",
  rate_limited: "errorRateLimited",
};

function readPending(): Pending | null {
  try {
    const raw = window.sessionStorage.getItem(TELEGRAM_LOGIN_STORAGE_KEY);
    if (!raw) return null;
    const p = JSON.parse(raw) as Partial<Pending>;
    if (typeof p.token !== "string" || typeof p.botURL !== "string" || typeof p.expiresAt !== "number") return null;
    if (!isTelegramDeepLink(p.botURL) || p.expiresAt <= Date.now()) return null;
    return { token: p.token, botURL: p.botURL, expiresAt: p.expiresAt };
  } catch {
    return null;
  }
}

function writePending(p: Pending | null) {
  try {
    if (p) window.sessionStorage.setItem(TELEGRAM_LOGIN_STORAGE_KEY, JSON.stringify(p));
    else window.sessionStorage.removeItem(TELEGRAM_LOGIN_STORAGE_KEY);
  } catch {
    /* storage blocked: only the resume-after-navigation nicety is lost */
  }
}

/** Only our own kind of link is ever opened: https://t.me/<bot>?start=login_… */
export function isTelegramDeepLink(url: string): boolean {
  try {
    const u = new URL(url);
    return u.protocol === "https:" && u.hostname === "t.me" && (u.searchParams.get("start") ?? "").startsWith("login_");
  } catch {
    return false;
  }
}

function isMobileBrowser(): boolean {
  return /Android|iPhone|iPad|iPod|Mobile/i.test(navigator.userAgent);
}

function formatLeft(ms: number): string {
  const s = Math.max(0, Math.ceil(ms / 1000));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
}

export function TelegramLogin({
  mode,
  onSuccess,
  onAvailability,
  ref,
}: {
  mode: "login" | "register";
  onSuccess: (result: TelegramLoginResult) => void | Promise<void>;
  /** false once the kill switch is known to be off and nothing is shown. */
  onAvailability?: (shown: boolean) => void;
  ref?: Ref<TelegramLoginHandle>;
}) {
  const t = useTranslations("TelegramLogin");
  const [phase, setPhase] = useState<Phase>("idle");
  const [pending, setPending] = useState<Pending | null>(null);
  const [errorKey, setErrorKey] = useState("errorNetwork");
  const [mobile, setMobile] = useState(false);
  const [now, setNow] = useState(() => Date.now());
  // The telegram_login kill switch. Assumed on until the public flags say
  // otherwise, so the button never flickers in for the usual case; a click
  // that beats the answer gets the server's 503 and the same copy.
  const [switchedOff, setSwitchedOff] = useState(false);
  const busy = useRef(false);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const onSuccessRef = useRef(onSuccess);
  useEffect(() => {
    onSuccessRef.current = onSuccess;
  }, [onSuccess]);

  useEffect(() => {
    setMobile(isMobileBrowser());
    const resumed = readPending();
    if (resumed) {
      setPending(resumed);
      setPhase("waiting");
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    void fetchTelegramLoginEnabled().then((on) => {
      if (!cancelled && !on) setSwitchedOff(true);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const finish = useCallback((next: Phase) => {
    writePending(null);
    setPending(null);
    setPhase(next);
  }, []);

  const start = useCallback(async () => {
    if (busy.current) return;
    busy.current = true;
    const onMobile = isMobileBrowser();
    // A tab opened inside the click survives popup blockers; it is pointed at
    // Telegram once the link exists. No opener: t.me must not reach back.
    let popup: Window | null = null;
    if (!onMobile) {
      try {
        popup = window.open("", "_blank");
        if (popup) popup.opener = null;
      } catch {
        popup = null;
      }
    }
    setPhase("starting");
    try {
      let res: Response;
      try {
        res = await fetch("/api/auth/telegram-login/start", { method: "POST" });
      } catch {
        popup?.close();
        setErrorKey("errorNetwork");
        setPhase("error");
        return;
      }
      const json = (await res.json().catch(() => null)) as
        | { data?: { bot_url?: string; token?: string; expires_in_sec?: number }; error?: { code?: string } }
        | null;
      const botURL = json?.data?.bot_url ?? "";
      const token = json?.data?.token ?? "";
      if (!res.ok || !token || !isTelegramDeepLink(botURL)) {
        popup?.close();
        setErrorKey(START_ERRORS[json?.error?.code ?? ""] ?? (res.status === 429 ? "errorRateLimited" : "errorNetwork"));
        setPhase("error");
        return;
      }
      const ttl = Math.min(300, Math.max(60, json?.data?.expires_in_sec ?? 300)) * 1000;
      const next = { token, botURL, expiresAt: Date.now() + ttl };
      writePending(next);
      setPending(next);
      setNow(Date.now());
      setPhase("waiting");
      if (popup) {
        popup.location.href = botURL;
      } else if (onMobile) {
        window.location.assign(botURL);
      }
    } finally {
      busy.current = false;
    }
  }, []);

  useImperativeHandle(ref, () => ({ start: () => void start() }), [start]);

  const complete = useCallback(
    async (token: string) => {
      setPhase("completing");
      let res: Response;
      try {
        res = await fetch("/api/auth/telegram-login/complete", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token }),
        });
      } catch {
        setPhase("waiting"); // the next poll retries
        return;
      }
      const json = (await res.json().catch(() => null)) as
        | { data?: { must_change_password?: boolean; created?: boolean; phone_masked?: unknown }; error?: { code?: string } }
        | null;
      if (res.status === 409) {
        setPhase("waiting");
        return;
      }
      if (!res.ok) {
        if (json?.error?.code === "account_blocked") finish("blocked");
        else if (res.status >= 500 || res.status === 429) setPhase("waiting");
        else finish("expired");
        return;
      }
      writePending(null);
      // Shown once on the first learner screen (SignedInNotice).
      rememberSignedInAs(json?.data?.phone_masked);
      await onSuccessRef.current({
        mustChangePassword: json?.data?.must_change_password === true,
        created: json?.data?.created === true,
      });
    },
    [finish]
  );

  // Poll while waiting: every 2 s, backing off on failures, at once when the
  // learner comes back to this tab from Telegram, and never past the expiry.
  useEffect(() => {
    if (phase !== "waiting" || !pending) return;
    let stopped = false;
    let timer: number | undefined;
    let delay = POLL_MS;
    let inFlight = false;
    const poll = async () => {
      if (stopped || inFlight) return;
      if (Date.now() > pending.expiresAt + COMPLETE_GRACE_MS) {
        finish("expired");
        return;
      }
      inFlight = true;
      try {
        const res = await fetch("/api/auth/telegram-login/status", {
          cache: "no-store",
          headers: { "X-Telegram-Login-Token": pending.token },
        });
        const json = (await res.json().catch(() => null)) as { data?: { state?: string } } | null;
        if (stopped) return;
        const state = res.ok ? json?.data?.state : undefined;
        delay = state ? POLL_MS : Math.min(delay * 2, MAX_POLL_BACKOFF_MS);
        if (state === "approved") {
          stopped = true;
          void complete(pending.token);
          return;
        }
        if (state === "cancelled") return finish("cancelled");
        if (state === "blocked") return finish("blocked");
        if (state === "invalid") return finish("expired");
      } catch {
        delay = Math.min(delay * 2, MAX_POLL_BACKOFF_MS);
      } finally {
        inFlight = false;
      }
      if (!stopped) timer = window.setTimeout(() => void poll(), delay);
    };
    const onVisible = () => {
      if (document.visibilityState !== "visible" || stopped) return;
      window.clearTimeout(timer);
      void poll();
    };
    void poll();
    document.addEventListener("visibilitychange", onVisible);
    const tick = window.setInterval(() => setNow(Date.now()), 1000);
    return () => {
      stopped = true;
      window.clearTimeout(timer);
      window.clearInterval(tick);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [phase, pending, complete, finish]);

  useEffect(() => {
    if (phase === "waiting" || phase === "cancelled" || phase === "expired" || phase === "blocked" || phase === "error") {
      headingRef.current?.focus();
    }
  }, [phase]);

  const hidden = phase === "idle" && switchedOff;
  useEffect(() => {
    onAvailability?.(!hidden);
  }, [hidden, onAvailability]);

  const label = mode === "register" ? t("registerButton") : t("loginButton");

  if (hidden) return null;

  if (phase === "idle" || phase === "starting") {
    return (
      <div className="space-y-1.5">
        <button
          type="button"
          onClick={() => void start()}
          disabled={phase === "starting"}
          aria-busy={phase === "starting"}
          // Telegram blue, darkened to keep white text at AA (4.9:1).
          className="flex min-h-12 w-full items-center justify-center gap-2 rounded-2xl bg-[#1f75bc] px-4 py-3 text-sm font-extrabold text-white shadow-sm transition-colors hover:bg-[#1a66a5] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:opacity-70 active:scale-[0.99]"
        >
          {phase === "starting" ? (
            <Loader2 aria-hidden="true" className="h-4 w-4 animate-spin motion-reduce:animate-none" />
          ) : (
            <Send aria-hidden="true" className="h-4 w-4" />
          )}
          {phase === "starting" ? t("starting") : label}
        </button>
        <p className="text-center text-[11px] font-semibold text-muted-foreground">{t("oneTapHint")}</p>
      </div>
    );
  }

  if (phase === "waiting" || phase === "completing") {
    const left = pending ? formatLeft(pending.expiresAt - now) : "";
    return (
      <section aria-labelledby="tglogin-title" className="space-y-4 rounded-2xl border border-[#1f75bc]/30 bg-[#1f75bc]/5 p-4">
        <div className="space-y-1.5">
          <h2
            id="tglogin-title"
            ref={headingRef}
            tabIndex={-1}
            className="flex items-center gap-2 font-display text-lg font-extrabold tracking-tight outline-none"
          >
            <Send aria-hidden="true" className="h-5 w-5 shrink-0 text-[#1f75bc] dark:text-[#5fb2f0]" />
            {t("waitingTitle")}
          </h2>
          <p className="text-sm leading-snug text-muted-foreground">{t("waitingHint")}</p>
        </div>
        {pending && phase === "waiting" && (
          <a
            href={pending.botURL}
            target="_blank"
            rel="noopener noreferrer"
            className="flex min-h-12 w-full items-center justify-center gap-2 rounded-2xl bg-[#1f75bc] px-4 py-3 text-sm font-extrabold text-white hover:bg-[#1a66a5] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            <ExternalLink aria-hidden="true" className="h-4 w-4" />
            {t("openTelegram")}
          </a>
        )}
        {pending && phase === "waiting" && !mobile && (
          <div className="flex flex-col items-center gap-2">
            <QrCode value={pending.botURL} label={t("qrAlt")} className="h-44 w-44 border border-border" />
            <p className="text-center text-xs font-semibold text-muted-foreground">{t("qrHint")}</p>
          </div>
        )}
        <div className="space-y-0.5 text-center">
          <p role="status" aria-live="polite" className="flex items-center justify-center gap-2 text-xs font-semibold text-muted-foreground">
            <Loader2 aria-hidden="true" className="h-3.5 w-3.5 shrink-0 animate-spin motion-reduce:animate-none" />
            {phase === "completing" ? t("completing") : t("waitingStatus")}
          </p>
          {/* Outside the live region: a ticking clock must not be read out every second. */}
          {phase === "waiting" && left && (
            <p className="text-[11px] font-semibold tabular-nums text-muted-foreground">{t("expiresIn", { time: left })}</p>
          )}
        </div>
        {phase === "waiting" && (
          <button
            type="button"
            onClick={() => finish("idle")}
            className="mx-auto block min-h-11 px-3 text-sm font-bold text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {t("cancel")}
          </button>
        )}
      </section>
    );
  }

  const notice =
    phase === "cancelled"
      ? { icon: XCircle, title: t("cancelledTitle"), body: t("cancelledBody"), retry: true }
      : phase === "expired"
        ? { icon: TimerOff, title: t("expiredTitle"), body: t("expiredBody"), retry: true }
        : phase === "blocked"
          ? { icon: ShieldAlert, title: t("blockedTitle"), body: t("blockedBody"), retry: false }
          : { icon: ShieldAlert, title: label, body: t(errorKey), retry: errorKey !== "errorUnavailable" };
  const Icon = notice.icon;
  return (
    <section aria-labelledby="tglogin-title" className="space-y-3 rounded-2xl border border-border bg-background/60 p-4">
      <h2
        id="tglogin-title"
        ref={headingRef}
        tabIndex={-1}
        className="flex items-center gap-2 font-display text-base font-extrabold outline-none"
      >
        <Icon aria-hidden="true" className={`h-5 w-5 shrink-0 ${phase === "blocked" ? "text-danger" : "text-muted-foreground"}`} />
        {notice.title}
      </h2>
      <p className="text-sm leading-snug text-muted-foreground">{notice.body}</p>
      {notice.retry && (
        <Button type="button" variant="outline" size="lg" className="w-full text-sm font-extrabold" onClick={() => void start()}>
          <RotateCw aria-hidden="true" className="mr-2 h-4 w-4" />
          {t("retry")}
        </Button>
      )}
    </section>
  );
}
