"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import {
  AlertTriangle,
  ImageOff,
  Play,
  RefreshCw,
  Search,
  X,
} from "lucide-react";
import { BackLink } from "@/components/layout/back-link";
import { useSignDetail, useSigns, type SignItem } from "@/hooks/use-signs";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

function practiceHref(sessionStartBase: string, code: string, count: number): string {
  const params = new URLSearchParams({
    mode: "practice",
    sign_id: code,
    count: String(Math.max(count, 1)),
  });
  return `${sessionStartBase}?${params.toString()}`;
}

export interface SignsPageProps {
  // Reused as-is under the login-free kiosk (frontend/src/app/[locale]/(kiosk)/station/signs/page.tsx):
  // this page has no dashboard or VIP surface to suppress — signs are pure
  // reference content — only two navigation targets (the back link and the
  // practice-session push) that must stay inside /station instead of the
  // learner app's gated /practice and /session/start routes.
  kiosk?: boolean;
}

export default function SignsPage({ kiosk = false }: SignsPageProps = {}) {
  const t = useTranslations("Signs");
  const locale = useLocale();
  const backHref = kiosk ? `/${locale}/station/practice` : `/${locale}/practice`;
  // /station/session/start (not /session/start): keeps the proxy.ts kiosk
  // exemption scoped to the /station/... namespace — see practice/page.tsx
  // for the same pattern.
  const sessionStartBase = kiosk ? `/${locale}/station/session/start` : `/${locale}/session/start`;
  const [activeGroup, setActiveGroup] = useState("all");
  const [search, setSearch] = useState("");
  const [activeModalSign, setActiveModalSign] = useState<SignItem | null>(null);

  const { signs, loading, error, refetch } = useSigns(locale, activeGroup, search);
  const {
    sign: signDetail,
    loading: detailLoading,
    error: detailError,
    refetch: refetchDetail,
  } = useSignDetail(activeModalSign?.code ?? null, locale);

  useEffect(() => {
    if (!activeModalSign) return;

    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setActiveModalSign(null);
    };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [activeModalSign]);

  const groups = [
    { code: "all", name: t("groupAll") },
    { code: "warning", name: t("groupWarning") },
    { code: "priority", name: t("groupPriority") },
    { code: "prohibiting", name: t("groupProhibitory") },
    { code: "mandatory", name: t("groupMandatory") },
    { code: "info", name: t("groupInformation") },
    { code: "service", name: t("groupService") },
    { code: "supplementary", name: t("groupSupplementary") },
  ];
  const hasActiveQuery = activeGroup !== "all" || search.trim().length > 0;
  const activeDetail =
    signDetail?.code === activeModalSign?.code ? signDetail : null;
  const modalImage = activeDetail?.image_url ?? activeModalSign?.image_url ?? null;
  const modalCount = activeModalSign?.question_count ?? 0;

  return (
    <main className="page-shell space-y-5 sm:space-y-6">
      <header className="flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-center">
        <div>
          <BackLink href={backHref} kiosk={kiosk}>{t("backToPractice")}</BackLink>
          <h1 className="font-display text-2xl font-bold tracking-tight">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>

        <label className="relative w-full sm:w-72">
          <span className="sr-only">{t("searchPlaceholder")}</span>
          <Search
            className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <input
            type="search"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t("searchPlaceholder")}
            className="field-input pl-9"
          />
        </label>
      </header>

      <div className="chip-scroll" aria-label={t("groupFilterLabel")}>
        {groups.map((group) => (
          <button
            key={group.code}
            type="button"
            onClick={() => setActiveGroup(group.code)}
            aria-pressed={activeGroup === group.code}
            className={`filter-chip ${
              activeGroup === group.code ? "filter-chip-active" : "filter-chip-idle"
            }`}
          >
            {group.name}
          </button>
        ))}
      </div>

      {loading && (
        <div className="flex min-h-40 items-center justify-center" role="status" aria-live="polite">
          <RefreshCw className="mr-2 h-5 w-5 animate-spin text-accent" aria-hidden="true" />
          <span className="text-sm text-muted-foreground">{t("loading")}</span>
        </div>
      )}

      {!loading && error && (
        <Card className="border-destructive/40 bg-destructive/5 p-6 text-center" role="alert">
          <AlertTriangle className="mx-auto mb-3 h-10 w-10 text-destructive" aria-hidden="true" />
          <p className="font-semibold">{t("loadError")}</p>
          <Button className="mt-5" variant="outline" onClick={() => void refetch()}>
            <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
            {t("retry")}
          </Button>
        </Card>
      )}

      {!loading && !error && signs.length === 0 && (
        <Card className="p-8 text-center text-sm text-muted-foreground" role="status">
          {hasActiveQuery ? t("noResults") : t("emptyCatalog")}
        </Card>
      )}

      {!loading && !error && signs.length > 0 && (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
          {signs.map((sign) => {
            const hasQuestions = sign.question_count > 0;
            const content = (
              <>
                {hasQuestions && (
                  <span className="absolute right-2 top-2 inline-flex min-h-7 min-w-7 items-center justify-center rounded-full bg-accent px-1.5 text-xs font-extrabold text-accent-foreground shadow-sm">
                    {t("questionCountBadge", { count: sign.question_count })}
                  </span>
                )}
                <div className="mb-2 flex h-20 w-20 items-center justify-center rounded-xl bg-black/5 p-2">
                  {sign.image_url ? (
                    // Dynamic media URLs are served by the backend and intentionally stay unoptimized.
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                      src={sign.image_url}
                      alt={hasQuestions ? "" : sign.name}
                      className="max-h-full max-w-full object-contain"
                    />
                  ) : (
                    <span className="flex flex-col items-center gap-1 text-xs text-muted-foreground">
                      <ImageOff className="h-6 w-6" aria-hidden="true" />
                      {t("imageUnavailable")}
                    </span>
                  )}
                </div>
                <span className="rounded-full bg-accent/10 px-2.5 py-0.5 text-xs font-extrabold text-accent">
                  {sign.code}
                </span>
                <span className="mt-2 line-clamp-2 text-sm font-bold text-foreground">{sign.name}</span>
              </>
            );

            return (
              <Card key={sign.code} className="surface-interactive relative overflow-hidden p-0 hover:border-accent">
                {hasQuestions ? (
                  <Link
                    href={practiceHref(sessionStartBase, sign.code, sign.question_count)}
                    className="flex min-h-44 w-full flex-col items-center justify-between p-4 text-center focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
                    aria-label={`${sign.code}. ${sign.name}. ${t("questionCountLabel", { count: sign.question_count })}`}
                  >
                    {content}
                  </Link>
                ) : (
                  <button
                    type="button"
                    onClick={() => setActiveModalSign(sign)}
                    className="flex min-h-44 w-full flex-col items-center justify-between p-4 text-center focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
                    aria-haspopup="dialog"
                  >
                    {content}
                  </button>
                )}
              </Card>
            );
          })}
        </div>
      )}

      {activeModalSign && (
        <div
          className="fixed inset-0 z-50 flex items-end justify-center bg-black/60 p-0 backdrop-blur-sm sm:items-center sm:p-4"
          role="dialog"
          aria-modal="true"
          aria-labelledby="sign-detail-title"
        >
          <Card className="relative max-h-[92vh] w-full max-w-lg space-y-4 overflow-y-auto rounded-b-none rounded-t-3xl p-5 sm:rounded-2xl sm:p-6">
            <button
              type="button"
              onClick={() => setActiveModalSign(null)}
              aria-label={t("close")}
              className="absolute right-4 top-4 flex min-h-11 min-w-11 items-center justify-center rounded-full border border-border text-muted-foreground hover:bg-card hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <X className="h-5 w-5" aria-hidden="true" />
            </button>

            <div className="flex items-center gap-3 pr-12">
              <span className="rounded-full bg-accent/20 px-3 py-1 text-xs font-extrabold text-accent">
                {activeModalSign.code}
              </span>
              <h2 id="sign-detail-title" className="font-display text-xl font-bold">
                {activeDetail?.name ?? activeModalSign.name}
              </h2>
            </div>

            <div className="flex min-h-52 items-center justify-center rounded-2xl bg-black/5 p-4">
              {modalImage ? (
                // Dynamic media URLs are served by the backend and intentionally stay unoptimized.
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={modalImage}
                  alt={activeDetail?.name ?? activeModalSign.name}
                  className="max-h-48 object-contain"
                />
              ) : (
                <span className="flex flex-col items-center gap-2 text-sm text-muted-foreground">
                  <ImageOff className="h-8 w-8" aria-hidden="true" />
                  {t("imageUnavailable")}
                </span>
              )}
            </div>

            <div className="rounded-xl border border-border bg-background/50 p-4 text-sm leading-relaxed text-foreground">
              <h3 className="mb-2 font-bold uppercase tracking-wider text-accent">
                {t("descriptionTitle")}
              </h3>
              {(detailLoading || (!activeDetail && !detailError)) && (
                <p className="text-muted-foreground" role="status">
                  {t("detailLoading")}
                </p>
              )}
              {!detailLoading && detailError && (
                <div role="alert">
                  <p>{t("detailLoadError")}</p>
                  <Button className="mt-3" variant="outline" size="sm" onClick={() => void refetchDetail()}>
                    <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
                    {t("retry")}
                  </Button>
                </div>
              )}
              {!detailLoading && !detailError && activeDetail && (
                <p>
                  {activeDetail.description.trim()
                    ? activeDetail.description
                    : t("descriptionUnavailable")}
                </p>
              )}
              <p className="mt-3 text-xs font-semibold text-muted-foreground">
                {modalCount > 0
                  ? t("questionCountLabel", { count: modalCount })
                  : t("noQuestionsForSign")}
              </p>
            </div>

            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button variant="outline" size="sm" onClick={() => setActiveModalSign(null)}>
                {t("close")}
              </Button>
              {modalCount > 0 && (
                <Link
                  href={practiceHref(sessionStartBase, activeModalSign.code, modalCount)}
                  className="inline-flex h-9 items-center justify-center rounded-xl bg-accent px-4 text-sm font-extrabold text-accent-foreground"
                >
                  <Play className="mr-2 h-4 w-4" aria-hidden="true" />
                  {t("practiceThisSign")}
                </Link>
              )}
            </div>
          </Card>
        </div>
      )}
    </main>
  );
}
