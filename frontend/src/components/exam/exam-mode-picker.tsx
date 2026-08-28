"use client";

import { useCallback, useEffect } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Award, ChevronRight, Clock, ListChecks, Lock, RotateCcw, XCircle } from "lucide-react";
import { useMeQuery } from "@/hooks/use-me";

/**
 * The two official exam varieties.
 *
 * `count` is the only thing the client gets to choose; the backend whitelists
 * it (session.ExamConfigFor) and derives the time limit and mistake budget
 * from it server-side. The minutes/errors here are display copies of
 * backend/internal/session/rules.go and must be changed together with it.
 */
const MODES = [
  {
    key: "standard",
    count: 20,
    minutes: 25,
    errors: 2,
    icon: Award,
    titleKey: "standardTitle",
    audienceKey: "standardAudience",
    accent: "border-emerald-500/50 hover:border-emerald-400 focus-visible:ring-emerald-400",
    badge: "bg-emerald-500/15 text-emerald-300",
    shortcut: "1",
  },
  {
    key: "restore",
    count: 50,
    minutes: 50,
    errors: 4,
    icon: RotateCcw,
    titleKey: "restoreTitle",
    audienceKey: "restoreAudience",
    accent: "border-amber-500/50 hover:border-amber-400 focus-visible:ring-amber-400",
    badge: "bg-amber-500/15 text-amber-300",
    shortcut: "2",
  },
] as const;

export interface ExamModePickerProps {
  /**
   * Reused as-is under the login-free kiosk
   * (frontend/src/app/[locale]/(kiosk)/station/exam/page.tsx): a walk-up
   * student has no learner routes to land on, so every destination stays under
   * /station/... and the VIP paywall is never offered.
   */
  kiosk?: boolean;
}

export function ExamModePicker({ kiosk = false }: ExamModePickerProps) {
  const t = useTranslations("ExamPicker");
  const locale = useLocale();
  const router = useRouter();
  const { data: me } = useMeQuery();

  // A station profile is always licensed and has no /premium route to be sent
  // to, so the kiosk never shows the lock.
  const locked = !kiosk && me?.vip?.active === false;

  const start = useCallback(
    (count: number) => {
      if (locked) {
        router.push(`/${locale}/premium`);
        return;
      }
      const base = kiosk ? `/${locale}/station/session/start` : `/${locale}/session/start`;
      router.push(`${base}?mode=exam&count=${count}`);
    },
    [kiosk, locale, locked, router]
  );

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      const mode = MODES.find((item) => item.shortcut === event.key);
      if (!mode) return;
      event.preventDefault();
      start(mode.count);
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [start]);

  return (
    <main className="relative flex min-h-screen flex-col items-center justify-center overflow-hidden bg-[#091726] px-4 py-10 text-white">
      {/* The same cube mesh the exam runner uses, so this reads as its front door. */}
      <div
        className="pointer-events-none absolute inset-0"
        style={{
          opacity: 0.12,
          backgroundImage: `
            linear-gradient(135deg, #1a3a6a 25%, transparent 25%),
            linear-gradient(225deg, #1a3a6a 25%, transparent 25%),
            linear-gradient(315deg, #1a3a6a 25%, transparent 25%),
            linear-gradient(45deg,  #1a3a6a 25%, transparent 25%)
          `,
          backgroundSize: "20px 20px",
          backgroundPosition: "0 0, 0 10px, 10px -10px, -10px 0px",
        }}
        aria-hidden="true"
      />

      <div className="relative z-10 w-full max-w-4xl">
        {/* Kiosk only: this page fills a classroom screen that has no sidebar,
            no browser chrome and no keyboard shortcut out, so without this a
            student who opened the exam by mistake is stranded. In the learner
            app the sidebar is already on screen. Styled against the picker's
            own dark palette rather than the app tokens, which do not apply
            here. */}
        {kiosk && (
          <Link
            href={`/${locale}/station`}
            className="mb-6 inline-flex min-h-14 items-center gap-2.5 rounded-2xl border-2 border-white/25 bg-white/10 px-5 text-base font-extrabold tracking-tight text-white shadow-lg transition-all hover:border-white/50 hover:bg-white/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white focus-visible:ring-offset-2 focus-visible:ring-offset-[#091726] active:translate-y-0.5 active:shadow-none"
          >
            <ArrowLeft aria-hidden="true" className="h-5 w-5 shrink-0" />
            {t("backHome")}
          </Link>
        )}

        <header className="text-center">
          <h1 className="font-display text-3xl font-black tracking-tight sm:text-4xl">{t("title")}</h1>
          <p className="mt-2 text-sm text-slate-300 sm:text-base">{t("subtitle")}</p>
        </header>

        <div className="mt-8 grid gap-4 sm:mt-10 lg:grid-cols-2 lg:gap-6">
          {MODES.map((mode) => {
            const Icon = mode.icon;
            return (
              <button
                key={mode.key}
                type="button"
                data-testid={`exam-mode-${mode.key}`}
                onClick={() => start(mode.count)}
                className={`group flex min-h-[14rem] flex-col rounded-2xl border-2 bg-[#0d2e4d]/80 p-5 text-left shadow-lg transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-[#091726] sm:p-6 ${mode.accent}`}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className={`flex h-12 w-12 shrink-0 items-center justify-center rounded-xl ${mode.badge}`}>
                    <Icon className="h-6 w-6" aria-hidden="true" />
                  </div>
                  <span
                    aria-hidden="true"
                    className="hidden rounded-md border border-[#2a4568] bg-[#081320] px-2 py-1 font-mono text-xs font-bold text-slate-300 lg:inline"
                  >
                    {mode.shortcut}
                  </span>
                </div>

                <h2 className="mt-4 font-display text-xl font-extrabold sm:text-2xl">{t(mode.titleKey)}</h2>
                <p className="mt-1 text-sm text-slate-300">{t(mode.audienceKey)}</p>

                <dl className="mt-4 space-y-1.5 text-sm font-semibold text-slate-200">
                  <div className="flex items-center gap-2">
                    <ListChecks className="h-4 w-4 shrink-0 text-slate-400" aria-hidden="true" />
                    <dd>{t("questionsMeta", { count: mode.count, minutes: mode.minutes })}</dd>
                  </div>
                  <div className="flex items-center gap-2">
                    <Clock className="h-4 w-4 shrink-0 text-slate-400" aria-hidden="true" />
                    <dd>{t("errorsMeta", { count: mode.errors })}</dd>
                  </div>
                  <div className="flex items-center gap-2">
                    <XCircle className="h-4 w-4 shrink-0 text-slate-400" aria-hidden="true" />
                    <dd>{t("stopMeta", { n: mode.errors + 1 })}</dd>
                  </div>
                </dl>

                <span className="mt-5 inline-flex min-h-14 w-full items-center justify-center gap-2 rounded-xl bg-[#183654] px-4 text-base font-extrabold transition-colors group-hover:bg-[#1f4a72]">
                  {locked ? (
                    <>
                      <Lock className="h-4 w-4" aria-hidden="true" />
                      {t("vipLocked")}
                    </>
                  ) : (
                    <>
                      {t("start")}
                      <ChevronRight className="h-5 w-5" aria-hidden="true" />
                    </>
                  )}
                </span>
              </button>
            );
          })}
        </div>

        <p className="mt-6 hidden text-center text-xs text-slate-400 lg:block">{t("keyboardHint")}</p>
      </div>
    </main>
  );
}
