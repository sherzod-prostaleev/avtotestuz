"use client";

import { Suspense, useEffect, useRef } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { LoaderCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  useSessionEngine,
  type SessionMode,
  type StartSessionOptions,
} from "@/hooks/use-session-engine";

const SESSION_MODES: SessionMode[] = [
  "variant",
  "exam",
  "practice",
  "mistakes",
  "grand_mock",
  "review",
  "placement",
];

function isSessionMode(value: string | null): value is SessionMode {
  return value !== null && SESSION_MODES.includes(value as SessionMode);
}

interface SessionStartContentProps {
  // Reused as-is under the login-free kiosk
  // (frontend/src/app/[locale]/(kiosk)/station/session/start/page.tsx): a
  // walk-up student has no dashboard/tickets/practice/premium under the
  // learner's login-gated namespace to land on, so every destination this
  // screen can push to must stay under /station/... instead.
  kiosk?: boolean;
}

function SessionStartContent({ kiosk = false }: SessionStartContentProps) {
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
      const variantFromParam = searchParams.get("variant_from");
      const variantToParam = searchParams.get("variant_to");
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
      const variantFrom = variantFromParam ? Number(variantFromParam) : NaN;
      const variantTo = variantToParam ? Number(variantToParam) : NaN;
      if (
        Number.isInteger(variantFrom) &&
        Number.isInteger(variantTo) &&
        variantFrom > 0 &&
        variantTo >= variantFrom
      ) {
        options.variant_from = variantFrom;
        options.variant_to = variantTo;
      }

      if (countParam) {
        const count = Number(countParam);
        if (Number.isInteger(count) && count > 0) options.question_count = count;
      }

      const session = await startSession(mode, options);
      if (session?.id) {
        router.replace(
          kiosk ? `/${locale}/station/session/${session.id}` : `/${locale}/session/${session.id}`
        );
      }
    }

    initSession();
  }, [searchParams, router, locale, startSession, kiosk]);

  if (error) {
    // Each recoverable server code gets its own message *and* its own exit, so
    // the button always leads somewhere the user can act on the problem —
    // 402 to the tariffs, 403 on grand mock back to the dashboard where the
    // card shows exactly how much study is still missing.
    let destination = kiosk ? `/${locale}/station/tickets` : `/${locale}/tickets`;
    let actionLabel = t("backToTickets");
    let message =
      error.code === "network_error" ? sessionT("networkError") : sessionT("genericError");

    switch (error.code) {
      case "vip_required":
        // No checkout entry point on the kiosk: a walk-up student must never
        // be one tap from VIP purchase, and /premium is login-gated anyway.
        if (kiosk) {
          destination = `/${locale}/station`;
          actionLabel = t("backToStation");
        } else {
          destination = `/${locale}/premium`;
          actionLabel = t("goToPremium");
        }
        message = t("vipRequired");
        break;
      case "previous_ticket_required":
        destination = kiosk ? `/${locale}/station/tickets` : `/${locale}/tickets`;
        actionLabel = t("backToTickets");
        message = t("previousTicketRequired");
        break;
      case "daily_limit_reached":
        destination = kiosk ? `/${locale}/station/practice` : `/${locale}/practice`;
        actionLabel = t("backToPractice");
        message = practiceT("dailyLimitReached");
        break;
      case "mock_not_eligible":
        destination = kiosk ? `/${locale}/station` : `/${locale}/dashboard`;
        actionLabel = kiosk ? t("backToStation") : t("backToDashboard");
        message = t("mockNotEligible");
        break;
      case "nothing_due":
        destination = kiosk ? `/${locale}/station/practice` : `/${locale}/practice`;
        actionLabel = t("backToPractice");
        message = t("nothingDue");
        break;
    }

    return (
      <Card className="w-full border-destructive/40 bg-destructive/5 p-6 text-center">
        <p className="font-display text-lg font-bold text-destructive">{t("errorTitle")}</p>
        <p className="mt-2 text-sm text-muted-foreground">{message}</p>
        <div className="sticky-cta-bar mt-5">
          <Button variant="game" className="w-full" onClick={() => router.push(destination)}>
            {actionLabel}
          </Button>
        </div>
      </Card>
    );
  }

  return (
    <Card className="flex w-full items-center justify-center gap-2 p-8 text-muted-foreground">
      <LoaderCircle className="h-5 w-5 animate-spin text-accent" aria-hidden="true" />
      <span className="animate-pulse text-sm font-semibold">{t("starting")}</span>
    </Card>
  );
}

export interface SessionStartPageProps {
  // See SessionStartContentProps: reused as-is under the login-free kiosk.
  kiosk?: boolean;
}

export default function SessionStartPage({ kiosk = false }: SessionStartPageProps = {}) {
  const t = useTranslations("SessionStart");

  return (
    <main className="page-shell-narrow flex min-h-[60vh] items-center justify-center">
      <div className="w-full max-w-md">
        <Suspense
          fallback={
            <Card className="flex items-center justify-center gap-2 p-8 text-muted-foreground">
              <LoaderCircle className="h-5 w-5 animate-spin text-accent" aria-hidden="true" />
              <span className="text-sm">{t("loading")}</span>
            </Card>
          }
        >
          <SessionStartContent kiosk={kiosk} />
        </Suspense>
      </div>
    </main>
  );
}
