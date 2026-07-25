"use client";

import { useState, type KeyboardEvent } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTickets, TicketItem } from "@/hooks/use-tickets";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { prefetchVariantDetail } from "@/lib/prefetch-variant";
import { ArrowLeft, Search, Lock, Star, Play, RefreshCw } from "lucide-react";

type FilterStatus = "all" | "completed" | "in_progress" | "locked";

function getTicketState(ticket: TicketItem) {
  const bestCorrect = ticket.best_correct ?? ticket.score ?? 0;
  const isCompleted = ticket.status === "completed" || bestCorrect >= 18;
  const isLocked = ticket.status === "locked" || ticket.unlocked === false;
  const attempts = ticket.attempts ?? (ticket.status !== "unstarted" ? 1 : 0);

  return { bestCorrect, isCompleted, isLocked, attempts };
}

export default function TicketsPage() {
  const t = useTranslations("Tickets");
  const locale = useLocale();
  const router = useRouter();
  const { tickets, loading, error, refetch } = useTickets();

  const [search, setSearch] = useState("");
  const [filterStatus, setFilterStatus] = useState<FilterStatus>("all");

  const filteredTickets = tickets.filter((ticket) => {
    const matchesSearch = search === "" || ticket.number.toString().includes(search);
    if (!matchesSearch) return false;
    const { isCompleted, isLocked, attempts } = getTicketState(ticket);

    if (filterStatus === "completed") return isCompleted;
    if (filterStatus === "in_progress") return attempts > 0 && !isCompleted;
    if (filterStatus === "locked") return isLocked;
    return true;
  });

  const ticketStates = tickets.map((ticket) => ({ ticket, ...getTicketState(ticket) }));
  const completedCount = ticketStates.filter(({ isCompleted }) => isCompleted).length;
  const lockedCount = ticketStates.filter(({ isLocked }) => isLocked).length;
  const inProgressCount = ticketStates.filter(({ attempts, isCompleted, isLocked }) => attempts > 0 && !isCompleted && !isLocked).length;
  const availableCount = ticketStates.filter(({ isLocked }) => !isLocked).length;
  const unstartedCount = ticketStates.filter(({ isLocked, attempts }) => !isLocked && attempts === 0).length;
  const nextTicket = ticketStates.find(({ isLocked, isCompleted }) => !isLocked && !isCompleted) ?? null;
  const completedPercent = tickets.length > 0 ? Math.min(100, Math.round((completedCount / tickets.length) * 100)) : 0;

  const handleStartTicket = (ticket: TicketItem) => {
    const { isLocked } = getTicketState(ticket);
    if (isLocked) {
      router.push(`/${locale}/premium`);
      return;
    }
    // Warm SW variant-detail cache for this ticket (offline re-read later).
    prefetchVariantDetail(ticket.number, locale);
    router.push(`/${locale}/session/start?mode=variant&variant_id=${ticket.number}`);
  };

  const handleTicketKeyDown = (event: KeyboardEvent<HTMLDivElement>, ticket: TicketItem) => {
    if (event.key !== "Enter" && event.key !== " ") return;
    event.preventDefault();
    handleStartTicket(ticket);
  };

  const filterTabs: Array<{ key: FilterStatus; label: string }> = [
    { key: "all", label: t("filterAll") },
    { key: "completed", label: t("filterCompleted") },
    { key: "in_progress", label: t("filterInProgress") },
    { key: "locked", label: t("lockedText") },
  ];

  return (
    <main className="page-shell space-y-6 sm:space-y-8">
      <header className="flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-center">
        <div>
          <Link href={`/${locale}/dashboard`} className="back-link">
            <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("backHome")}
          </Link>
          <h1 className="font-display text-2xl font-bold tracking-tight">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>

        <div className="relative w-full sm:w-64">
          <Search aria-hidden="true" className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("searchPlaceholder")}
            aria-label={t("searchLabel")}
            className="field-input pl-9"
          />
        </div>
      </header>

      <section className="grid gap-4 lg:grid-cols-[1.15fr_0.85fr]">
        <Card className="overflow-hidden border-accent/20 bg-card p-5 md:p-8">
          <div className="flex flex-col gap-6">
            <div className="flex flex-wrap items-center gap-2">
              <span className="rounded-md bg-accent/15 px-3 py-1 text-[11px] font-extrabold text-accent">
                {t("statTotal", { count: tickets.length })}
              </span>
              <span className="rounded-md bg-gold/15 px-3 py-1 text-[11px] font-extrabold text-gold">
                {t("statCompleted", { count: completedCount })}
              </span>
              <span className="rounded-md bg-success/15 px-3 py-1 text-[11px] font-extrabold text-success">
                {t("statInProgress", { count: inProgressCount })}
              </span>
              <span className="rounded-md bg-muted px-3 py-1 text-[11px] font-extrabold text-muted-foreground">
                {t("statLocked", { count: lockedCount })}
              </span>
            </div>

            <div className="space-y-3">
              <div className="inline-flex items-center rounded-md border border-border bg-background px-3 py-1 text-[11px] font-bold text-muted-foreground">
                {t("ticketsProgress", { done: completedCount, total: tickets.length })}
              </div>
              <h2 className="font-display text-xl font-extrabold tracking-tight md:text-2xl">
                {t("heroTitle")}
              </h2>
              <p className="max-w-xl text-sm text-muted-foreground">
                {t("heroDescription")}
              </p>
              <p className="max-w-xl text-xs leading-5 text-muted-foreground/90">
                {t("leftoverNote")}
              </p>
              <div
                className="h-2 w-full overflow-hidden rounded-full bg-border"
                role="progressbar"
                aria-valuenow={completedPercent}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-label={t("ticketsProgress", { done: completedCount, total: tickets.length })}
              >
                <div
                  className="h-full rounded-full bg-accent transition-all duration-500"
                  style={{ width: `${completedPercent}%` }}
                />
              </div>
            </div>
          </div>
        </Card>

        <Card className="flex flex-col justify-between bg-card p-5 md:p-8">
          <div className="flex items-start justify-between gap-4">
            <div>
              <p className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground">
                {t("nextTicketLabel")}
              </p>
              {loading ? (
                // Same footprint skeleton: never flash "all done" before data lands.
                <>
                  <span aria-hidden="true" className="mt-2 block h-9 w-32 animate-pulse rounded-lg bg-border/60" />
                  <span aria-hidden="true" className="mt-2 block h-4 w-40 animate-pulse rounded bg-border/60" />
                </>
              ) : (
                <>
                  <h3 className="mt-2 font-display text-3xl font-black tracking-tight tabular-nums">
                    {nextTicket ? t("ticketNumber", { number: nextTicket.ticket.number }) : t("allDoneTitle")}
                  </h3>
                  <p className="mt-2 text-xs text-muted-foreground">
                    {nextTicket
                      ? t("nextTicketBest", {
                          best: nextTicket.bestCorrect,
                          total: nextTicket.ticket.total_questions ?? 20,
                        })
                      : t("heroEmpty")}
                  </p>
                </>
              )}
            </div>
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-accent/15 text-accent">
              <Play aria-hidden="true" className="h-6 w-6" />
            </div>
          </div>

          <div className="mt-5 flex flex-wrap gap-2 text-[11px] font-bold">
            <span className="rounded-md bg-background px-3 py-1 text-muted-foreground">
              {t("statAvailable", { count: availableCount })}
            </span>
            <span className="rounded-md bg-background px-3 py-1 text-muted-foreground">
              {t("statUnstarted", { count: unstartedCount })}
            </span>
          </div>

          {/* Desktop/tablet CTA lives in the hero card; the mobile twin is a
              page-end sticky bar so it stays in the thumb zone over the grid. */}
          <div className="mt-5 hidden sm:block">
            {loading ? (
              <span aria-hidden="true" className="block h-12 w-full animate-pulse rounded-2xl bg-border/60" />
            ) : nextTicket ? (
              <Button
                type="button"
                variant="game"
                size="lg"
                className="w-full"
                onClick={() => handleStartTicket(nextTicket.ticket)}
              >
                {t("solve")}
              </Button>
            ) : (
              <Button type="button" variant="outline" size="lg" className="w-full" onClick={() => router.push(`/${locale}/practice`)}>
                {t("goToPractice")}
              </Button>
            )}
          </div>
        </Card>
      </section>

      <div className="chip-scroll" role="group" aria-label={t("title")}>
        {filterTabs.map((tab) => (
          <button
            type="button"
            key={tab.key}
            onClick={() => setFilterStatus(tab.key)}
            aria-pressed={filterStatus === tab.key}
            className={`filter-chip ${
              filterStatus === tab.key ? "filter-chip-active" : "filter-chip-idle"
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {error && (
        <div role="alert" className="rounded-2xl border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive font-medium">
          <p>{t("loadError")}</p>
          <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => void refetch()}>
            <RefreshCw aria-hidden="true" className="mr-2 h-4 w-4" /> {t("retry")}
          </Button>
        </div>
      )}

      {/* 61 Tickets Grid */}
      {loading ? (
        <div role="status" className="py-12 text-center text-sm text-muted-foreground animate-pulse">{t("loading")}</div>
      ) : filteredTickets.length === 0 ? (
        <div className="py-12 text-center text-sm text-muted-foreground">{t("emptyFiltered")}</div>
      ) : (
        <div className="grid gap-4 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
          {filteredTickets.map((ticket) => {
            const { bestCorrect, isCompleted, isLocked, attempts } = getTicketState(ticket);

            return (
              <Card
                key={ticket.number}
                onClick={() => handleStartTicket(ticket)}
                onKeyDown={(event) => handleTicketKeyDown(event, ticket)}
                role="button"
                tabIndex={0}
                aria-label={t("openTicket", { number: ticket.number })}
                className={`relative flex min-h-[9.5rem] cursor-pointer flex-col items-center justify-between p-4 text-center transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                  isCompleted
                    ? "border-gold/40 bg-gold/5"
                    : isLocked
                    ? "border-border bg-card opacity-70"
                    : "border-accent/30 hover:border-accent"
                }`}
              >
                {/* Top Badge */}
                <div className="w-full flex items-center justify-between text-[11px] font-extrabold text-muted-foreground">
                  <span>{t("ticketNumber", { number: ticket.number })}</span>
                  {isLocked ? (
                    <Lock aria-hidden="true" className="h-3.5 w-3.5 text-muted-foreground" />
                  ) : isCompleted ? (
                    <Star aria-hidden="true" className="h-3.5 w-3.5 fill-gold text-gold" />
                  ) : null}
                </div>

                {/* Main Ticket Icon Number */}
                <div className="my-3 flex h-14 w-14 items-center justify-center rounded-2xl bg-card border border-border font-display text-2xl font-black text-foreground shadow-sm">
                  {ticket.number}
                </div>

                {/* Bottom Status / Score */}
                <div className="w-full">
                  {isLocked ? (
                    <span className="text-[10px] font-bold text-muted-foreground">{t("vipRequiredShort")}</span>
                  ) : isCompleted ? (
                    <span className="inline-flex items-center gap-1 rounded-full bg-gold/20 px-2 py-0.5 text-[11px] font-extrabold text-gold">
                      {t("scoreShort", { score: bestCorrect, total: ticket.total_questions ?? 20 })}
                    </span>
                  ) : attempts > 0 ? (
                    <span className="text-[11px] font-bold text-accent">
                      {t("scoreShort", { score: bestCorrect, total: ticket.total_questions ?? 20 })}
                    </span>
                  ) : (
                    <span className="inline-flex h-9 w-full items-center justify-center rounded-xl border-b-4 border-accent-shadow bg-accent px-3 text-[11px] font-bold text-accent-foreground shadow-3d">
                      <Play aria-hidden="true" className="mr-1 h-3 w-3" /> {t("solve")}
                    </span>
                  )}
                </div>
              </Card>
            );
          })}
        </div>
      )}

      {!loading && !error && (
        <div className="sticky-cta-bar sm:hidden">
          {nextTicket ? (
            <Button
              type="button"
              variant="game"
              size="lg"
              className="w-full"
              onClick={() => handleStartTicket(nextTicket.ticket)}
            >
              {t("solve")}
            </Button>
          ) : (
            <Button type="button" variant="outline" size="lg" className="w-full" onClick={() => router.push(`/${locale}/practice`)}>
              {t("goToPractice")}
            </Button>
          )}
        </div>
      )}
    </main>
  );
}
