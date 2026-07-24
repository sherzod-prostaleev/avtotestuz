"use client";

import { Suspense, useEffect, useRef } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import {
  useSessionEngine,
  type SessionMode,
  type StartSessionOptions,
} from "@/hooks/use-session-engine";

const SESSION_MODES: SessionMode[] = ["variant", "exam", "practice", "mistakes", "grand_mock"];

function isSessionMode(value: string | null): value is SessionMode {
  return value !== null && SESSION_MODES.includes(value as SessionMode);
}

function SessionStartContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const locale = useLocale();
  const t = useTranslations("SessionStart");
  const practiceT = useTranslations("Practice");
  const sessionT = useTranslations("Session");
  const { startSession, error } = useSessionEngine();
  // Keyed by the request itself, not a bare boolean: several sidebar entries
  // point at this route with different params, and App Router reuses the
  // mounted component across those navigations. A boolean guard would swallow
  // every start after the first one, leaving the user on a stale screen.
  const startedForRef = useRef<string | null>(null);

  useEffect(() => {
    const requestKey = searchParams.toString();
    if (startedForRef.current === requestKey) return;
    startedForRef.current = requestKey;

    async function initSession() {
      const modeParam = searchParams.get("mode");
      const variantParam = searchParams.get("variant_id");
      const categoryParam = searchParams.get("category_id");
      const signParam = searchParams.get("sign_id");
      const hasImageParam = searchParams.get("has_image");
      const countParam = searchParams.get("count");

      const mode: SessionMode = isSessionMode(modeParam) ? modeParam : variantParam ? "variant" : "exam";
      const options: StartSessionOptions = { locale };

      if (variantParam) options.variant_id = variantParam;
      if (categoryParam) options.category_id = categoryParam;
      if (signParam) options.sign_id = signParam;
      // Only "true"/"false" count; anything else leaves the selector unused so
      // a malformed URL cannot silently narrow the question pool.
      if (hasImageParam === "true" || hasImageParam === "false") {
        options.has_image = hasImageParam === "true";
      }

      if (countParam) {
        const count = Number(countParam);
        if (Number.isInteger(count) && count > 0) options.question_count = count;
      }

      const session = await startSession(mode, options);
      if (session?.id) {
        router.replace(`/${locale}/session/${session.id}`);
      }
    }

    initSession();
  }, [searchParams, router, locale, startSession]);

  if (error) {
    const isVipRequired = error.code === "vip_required";
    const isDailyLimitReached = error.code === "daily_limit_reached";
    const destination = isVipRequired
      ? `/${locale}/premium`
      : isDailyLimitReached
        ? `/${locale}/practice`
        : `/${locale}/tickets`;
    const actionLabel = isVipRequired
      ? t("goToPremium")
      : isDailyLimitReached
        ? t("backToPractice")
        : t("backToTickets");
    const message = isVipRequired
      ? t("vipRequired")
      : isDailyLimitReached
        ? practiceT("dailyLimitReached")
        : error.code === "network_error"
          ? sessionT("networkError")
          : sessionT("genericError");

    return (
      <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-6 text-center text-destructive">
        <p className="font-bold">{t("errorTitle")}</p>
        <p className="mt-1 text-sm">{message}</p>
        <button
          onClick={() => router.push(destination)}
          className="mt-4 rounded bg-accent px-4 py-2 text-xs font-bold text-accent-foreground"
        >
          {actionLabel}
        </button>
      </div>
    );
  }

  return <div className="text-center text-muted-foreground animate-pulse">{t("starting")}</div>;
}

export default function SessionStartPage() {
  const t = useTranslations("SessionStart");

  return (
    <main className="mx-auto flex min-h-[60vh] max-w-md items-center justify-center p-4">
      <Suspense fallback={<div className="text-center text-muted-foreground">{t("loading")}</div>}>
        <SessionStartContent />
      </Suspense>
    </main>
  );
}
