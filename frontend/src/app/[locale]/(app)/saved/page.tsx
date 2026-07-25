"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft, Bookmark, CalendarDays, ImageIcon, Trash2 } from "lucide-react";
import { apiDelete, apiGet } from "@/lib/api-client";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

interface SavedItemDTO {
  question_id: string;
  created_at: string;
}

interface QuestionDTO {
  id: string;
  category_code: string;
  text: string;
  image_url: string | null;
}

interface SavedQuestion {
  questionId: string;
  createdAt: string;
  question: QuestionDTO;
}

type LoadStatus = "loading" | "ready" | "error";

import { formatDateShort } from "@/lib/date-format";

function formatSavedDate(value: string): string {
  return formatDateShort(value);
}

export default function SavedPage() {
  const locale = useLocale();
  const t = useTranslations("Saved");
  const [items, setItems] = useState<SavedQuestion[]>([]);
  const [status, setStatus] = useState<LoadStatus>("loading");
  const [deleting, setDeleting] = useState<Set<string>>(new Set());
  const [deleteErrors, setDeleteErrors] = useState<Record<string, boolean>>({});
  const deletingRef = useRef<Set<string>>(new Set());

  const loadSaved = useCallback(async () => {
    setStatus("loading");
    setDeleteErrors({});
    try {
      const saved = await apiGet<SavedItemDTO[]>("me/saved");
      const loaded = await Promise.all(
        saved.map(async (item) => ({
          questionId: item.question_id,
          createdAt: item.created_at,
          question: await apiGet<QuestionDTO>(
            `questions/${encodeURIComponent(item.question_id)}?locale=${encodeURIComponent(locale)}`
          ),
        }))
      );
      setItems(loaded);
      setStatus("ready");
    } catch {
      setItems([]);
      setStatus("error");
    }
  }, [locale]);

  useEffect(() => {
    void loadSaved();
  }, [loadSaved]);

  const removeSaved = async (questionId: string) => {
    if (deletingRef.current.has(questionId)) return;

    deletingRef.current.add(questionId);
    setDeleting((current) => new Set(current).add(questionId));
    setDeleteErrors((current) => ({ ...current, [questionId]: false }));

    try {
      await apiDelete<{ ok: boolean }>(`me/saved/${encodeURIComponent(questionId)}`);
      setItems((current) => current.filter((item) => item.questionId !== questionId));
    } catch {
      setDeleteErrors((current) => ({ ...current, [questionId]: true }));
    } finally {
      deletingRef.current.delete(questionId);
      setDeleting((current) => {
        const next = new Set(current);
        next.delete(questionId);
        return next;
      });
    }
  };

  return (
    <main className="page-shell-tight space-y-6 sm:space-y-8">
      <header>
        <Link href={`/${locale}/dashboard`} className="back-link">
          <ArrowLeft className="h-4 w-4" /> {t("backToDashboard")}
        </Link>
        <div className="flex items-center gap-3">
          <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-accent/15 text-accent">
            <Bookmark className="h-6 w-6" />
          </div>
          <div>
            <h1 className="font-display text-2xl font-extrabold tracking-tight sm:text-3xl">{t("title")}</h1>
            <p className="mt-1 text-sm text-muted-foreground">{t("subtitle")}</p>
          </div>
        </div>
      </header>

      <section className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
        <Card className="overflow-hidden border-gold/30 bg-card p-5 md:p-8">
          <div className="flex flex-col gap-5">
            <div className="flex flex-wrap items-center gap-2">
              <span className="rounded-md bg-gold/15 px-3 py-1 text-[11px] font-extrabold text-gold">
                {t("navLabel")}
              </span>
              <span className="rounded-md bg-accent/15 px-3 py-1 text-[11px] font-extrabold text-accent">
                {t("countLabel", { count: items.length })}
              </span>
            </div>
            <div className="space-y-3">
              <h2 className="font-display text-xl font-extrabold tracking-tight md:text-2xl">
                {t("heroTitle")}
              </h2>
              <p className="max-w-2xl text-sm text-muted-foreground">
                {t("heroBody")}
              </p>
            </div>
          </div>
        </Card>

        <Card className="flex flex-col justify-between bg-card p-5 md:p-8">
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="rounded-2xl border border-border bg-background p-4">
              <p className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground">
                {t("navLabel")}
              </p>
              <p className="mt-2 font-display text-2xl font-black tabular-nums">{items.length}</p>
            </div>
            <div className="rounded-2xl border border-border bg-background p-4">
              <p className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground">
                {t("browseTickets")}
              </p>
              <p className="mt-2 text-sm font-semibold text-foreground">{t("browseHint")}</p>
            </div>
          </div>
          <div className="sticky-cta-bar mt-5">
            <Link href={`/${locale}/tickets`} className="block w-full">
              <Button variant="game" size="sm" className="w-full">
                {t("browseTickets")}
              </Button>
            </Link>
          </div>
        </Card>
      </section>

      {status === "loading" && (
        <Card className="p-10 text-center text-sm text-muted-foreground animate-pulse">
          {t("loading")}
        </Card>
      )}

      {status === "error" && (
        <Card role="alert" className="space-y-4 border-destructive/40 bg-destructive/5 p-8 text-center">
          <p className="font-semibold text-destructive">{t("loadError")}</p>
          <Button variant="outline" size="sm" onClick={() => void loadSaved()}>
            {t("retry")}
          </Button>
        </Card>
      )}

      {status === "ready" && items.length === 0 && (
        <Card className="space-y-4 p-10 text-center">
          <Bookmark className="mx-auto h-10 w-10 text-muted-foreground" />
          <div>
            <h2 className="font-display text-xl font-bold">{t("emptyTitle")}</h2>
            <p className="mt-1 text-sm text-muted-foreground">{t("emptyDescription")}</p>
          </div>
          <Link href={`/${locale}/tickets`}>
            <Button variant="game" size="sm">{t("browseTickets")}</Button>
          </Link>
        </Card>
      )}

      {status === "ready" && items.length > 0 && (
        <section className="grid gap-4 sm:grid-cols-2">
          {items.map((item, index) => {
            const isDeleting = deleting.has(item.questionId);
            const deleteFailed = deleteErrors[item.questionId] === true;

            return (
              <Card key={item.questionId} className="flex h-full flex-col overflow-hidden">
                {item.question.image_url && (
                  <div className="border-b border-border bg-background/60 p-3">
                    {/* Dynamic media URLs are served by the backend and intentionally stay unoptimized. */}
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={item.question.image_url}
                      alt={t("imageAlt", { number: index + 1 })}
                      className="mx-auto max-h-52 w-full rounded-xl object-contain"
                    />
                  </div>
                )}

                <div className="flex flex-1 flex-col gap-4 p-5">
                  <div className="flex flex-wrap items-center gap-2 text-[11px] font-semibold text-muted-foreground">
                    <span className="inline-flex items-center gap-1 rounded-full bg-accent/10 px-2.5 py-1 text-accent">
                      <ImageIcon className="h-3.5 w-3.5" />
                      <span>{t("categoryLabel")}</span>
                      <span>{item.question.category_code}</span>
                    </span>
                    <span className="inline-flex items-center gap-1">
                      <CalendarDays className="h-3.5 w-3.5" />
                      <span>{t("savedAt")}</span>
                      <time dateTime={item.createdAt}>{formatSavedDate(item.createdAt)}</time>
                    </span>
                  </div>

                  <h2 className="font-display text-base font-bold leading-relaxed">{item.question.text}</h2>

                  <div className="mt-auto space-y-2 border-t border-border pt-4">
                    {deleteFailed && <p className="text-xs text-destructive">{t("removeError")}</p>}
                    <Button
                      variant="outline"
                      size="sm"
                      className="w-full"
                      disabled={isDeleting}
                      onClick={() => void removeSaved(item.questionId)}
                    >
                      <Trash2 className="mr-2 h-4 w-4" />
                      {isDeleting
                        ? t("removing")
                        : deleteFailed
                          ? t("retryRemove")
                          : t("remove")}
                    </Button>
                  </div>
                </div>
              </Card>
            );
          })}
        </section>
      )}
    </main>
  );
}
