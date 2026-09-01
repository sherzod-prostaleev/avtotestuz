"use client";

import { useState, type KeyboardEvent } from "react";
import { useTranslations, useLocale } from "next-intl";
import { useRouter } from "next/navigation";
import { useTickets, TicketItem } from "@/hooks/use-tickets";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { prefetchVariantDetail } from "@/lib/prefetch-variant";
import { OFFICIAL_TICKET_COUNT } from "@/lib/content-counts";
import { Check, Lock, Play, RefreshCw, Search, Star, X } from "lucide-react";
import { BackLink } from "@/components/layout/back-link";


type FilterStatus = "all" | "completed" | "in_progress" | "locked";

function getTicketState(ticket: TicketItem) {
  const bestCorrect = ticket.best_correct ?? ticket.score ?? 0;
  // Align with backend unlock_threshold_correct / completed_at — not exam pass (≥18).
  const isCompleted = ticket.status === "completed" || Boolean(ticket.completed_at);
  const isLocked = ticket.status === "locked" || ticket.unlocked === false;
  const attempts = ticket.attempts ?? (ticket.status !== "unstarted" ? 1 : 0);
  const totalQuestions = ticket.total_questions ?? 20;
  const progressPercent = Math.min(100, Math.round((bestCorrect / totalQuestions) * 100));

  return { bestCorrect, isCompleted, isLocked, attempts, totalQuestions, progressPercent };
}

function ticketTileClass(isCompleted: boolean, isLocked: boolean, attempts: number) {
  if (isLocked) return "ticket-tile ticket-tile-locked";
  if (isCompleted) return "ticket-tile ticket-tile-done";
  if (attempts > 0) return "ticket-tile ticket-tile-progress";
  return "ticket-tile ticket-tile-open";
}

export interface TicketsPageProps {
  // Reused as-is under the login-free kiosk (frontend/src/app/[locale]/(kiosk)/station/tickets/page.tsx):
  // a walk-up student has no dashboard or VIP checkout to go back to, so
  // those two surfaces must not appear when kiosk is true.
  kiosk?: boolean;
}

export default function TicketsPage({ kiosk = false }: TicketsPageProps = {}) {
  const t = useTranslations("Tickets");
  const locale = useLocale();
  const router = useRouter();
  const { tickets, loading, error, refetch } = useTickets();
  // The page already has the real list — count it rather than quoting a number
  // that goes stale the next time a bilet is imported.
  const ticketCount = tickets.length || OFFICIAL_TICKET_COUNT;
  const backHref = kiosk ? `/${locale}/station` : `/${locale}/dashboard`;
  const practiceHref = kiosk ? `/${locale}/station/practice` : `/${locale}/practice`;
  // /station/session/start (not /session/start): keeps the proxy.ts kiosk
  // exemption scoped to the /station/... namespace — see practice/page.tsx
  // for the same pattern.
  const sessionStartBase = kiosk ? `/${locale}/station/session/start` : `/${locale}/session/start`;

  const [search, setSearch] = useState("");
  // The phone header has no room for a search row; it opens from a button.
  const [searchOpen, setSearchOpen] = useState(false);
  const [filterStatus, setFilterStatus] = useState<FilterStatus>("all");
  const [lockNotice, setLockNotice] = useState<string | null>(null);

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
      if (ticket.lock_reason === "prev_required") {
        setLockNotice(t("lockedPrevRequired"));
        return;
      }
      // No checkout entry point on the kiosk: a walk-up student must never
      // be one tap from VIP purchase. Show the same locked notice instead
      // of the learner app's redirect to /premium.
      if (kiosk) {
        setLockNotice(t("vipRequiredShort"));
        return;
      }
      setLockNotice(null);
      router.push(`/${locale}/premium`); // kiosk-safe: unreachable in kiosk mode — the if (kiosk) block above already returns before this line runs
      return;
    }
    setLockNotice(null);
    // Warm SW variant-detail cache for this ticket (offline re-read later).
    prefetchVariantDetail(ticket.number, locale);
    router.push(`${sessionStartBase}?mode=variant&variant_id=${ticket.number}`);
  };

  const handleTicketKeyDown = (event: KeyboardEvent<HTMLDivElement>, ticket: TicketItem) => {
    if (event.key !== "Enter" && event.key !== " ") return;
    event.preventDefault();
    handleStartTicket(ticket);
  };

  const filterTabs: Array<{ key: FilterStatus; label: string; count: number }> = [
    { key: "all", label: t("filterAll"), count: tickets.length },
    { key: "completed", label: t("filterCompleted"), count: completedCount },
    { key: "in_progress", label: t("filterInProgress"), count: inProgressCount },
    { key: "locked", label: t("lockedText"), count: tickets.length - availableCount },
  ];

  return (
    <main className="page-shell space-y-6 max-md:space-y-2.5 sm:space-y-8">
      <header className="flex flex-col items-start justify-between gap-4 max-md:flex-row max-md:items-center sm:flex-row sm:items-center">
        <div className="min-w-0 flex-1">
          <span className="max-md:hidden">
            <BackLink href={backHref} kiosk={kiosk}>{t("backHome")}</BackLink>
          </span>
          <h1 className="font-display text-2xl font-bold tracking-tight">{t("title")}</h1>
          <p className="text-sm text-muted-foreground max-md:hidden">{t("subtitle", { count: ticketCount })}</p>
          <p className="text-sm text-muted-foreground md:hidden">
            {t("subtitleProgress", { total: tickets.length, done: completedCount })}
          </p>
        </div>

        <div className={`relative w-full sm:w-64 ${searchOpen ? "" : "max-md:hidden"}`}>
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

        <button
          type="button"
          aria-label={t("searchLabel")}
          aria-expanded={searchOpen}
          onClick={() => setSearchOpen((open) => !open)}
          className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-border bg-card text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring md:hidden"
        >
          {searchOpen ? <X aria-hidden="true" className="h-5 w-5" /> : <Search aria-hidden="true" className="h-5 w-5" />}
        </button>
      </header>

      <section className="grid gap-4 max-md:hidden lg:grid-cols-[1.15fr_0.85fr]">
        <Card className="asphalt-hero overflow-hidden border-accent/20 p-5 md:p-8">
          <div className="flex flex-col gap-6">
            <div className="flex flex-wrap items-center gap-2">
              <span className="rounded-md bg-accent/15 px-3 py-1.5 text-sm font-extrabold text-accent">
                {t("statTotal", { count: tickets.length })}
              </span>
              <span className="rounded-md bg-gold/15 px-3 py-1.5 text-sm font-extrabold text-gold">
                {t("statCompleted", { count: completedCount })}
              </span>
              <span className="rounded-md bg-success/15 px-3 py-1.5 text-sm font-extrabold text-success">
                {t("statInProgress", { count: inProgressCount })}
              </span>
              <span className="rounded-md bg-muted px-3 py-1.5 text-sm font-extrabold text-muted-foreground">
                {t("statLocked", { count: lockedCount })}
              </span>
            </div>

            <div className="space-y-3">
              <div className="inline-flex items-center rounded-md border border-border/80 bg-background/60 px-3 py-1.5 text-sm font-bold text-muted-foreground backdrop-blur-sm">
                {t("ticketsProgress", { done: completedCount, total: tickets.length })}
              </div>
              <h2 className="font-display text-2xl font-extrabold tracking-tight md:text-3xl">
                {t("heroTitle")}
              </h2>
              <p className="max-w-xl text-base leading-relaxed text-muted-foreground">
                {t("heroDescription")}
              </p>
              <p className="max-w-xl text-sm font-semibold text-accent">{t("unlockHint")}</p>
              <div
                className="ticket-progress-track h-2"
                role="progressbar"
                aria-valuenow={completedPercent}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-label={t("ticketsProgress", { done: completedCount, total: tickets.length })}
              >
                <div
                  className="ticket-progress-fill animate-grow-bar"
                  style={{ width: `${completedPercent}%` }}
                />
              </div>
            </div>
          </div>
        </Card>

        <Card className="asphalt-hero relative flex flex-col justify-between overflow-hidden border-accent/20 p-5 md:p-8">
          <span
            aria-hidden="true"
            className="absolute inset-y-4 left-0 w-1 rounded-full bg-gradient-to-b from-accent to-accent-shadow"
          />
          <div className="flex items-start justify-between gap-4 pl-2">
            <div>
              <p className="text-xs font-bold uppercase tracking-[0.16em] text-muted-foreground">
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
                  <p className="mt-2 text-sm text-muted-foreground">
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

          <div className="mt-5 flex flex-wrap gap-2 pl-2 text-sm font-bold">
            <span className="rounded-md bg-background/70 px-3 py-1.5 text-muted-foreground backdrop-blur-sm">
              {t("statAvailable", { count: availableCount })}
            </span>
            <span className="rounded-md bg-background/70 px-3 py-1.5 text-muted-foreground backdrop-blur-sm">
              {t("statUnstarted", { count: unstartedCount })}
            </span>
          </div>

          {/* Desktop/tablet CTA lives in the hero card; the mobile twin is a
              page-end sticky bar so it stays in the thumb zone over the grid. */}
          <div className="mt-5 hidden pl-2 sm:block">
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
              <Button type="button" variant="outline" size="lg" className="w-full" onClick={() => router.push(practiceHref)}>
                {t("goToPractice")}
              </Button>
            )}
          </div>
        </Card>
      </section>

      {/* One recommendation, because nobody knows which of 64 to open. The wide
          layout carries the same call to action inside its hero card. */}
      {!loading && nextTicket && (
        <button
          type="button"
          onClick={() => handleStartTicket(nextTicket.ticket)}
          className="flex w-full items-center gap-3 rounded-xl border border-accent/35 bg-accent/[0.06] p-2 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring md:hidden"
        >
          <span className="min-w-0 flex-1">
            <span className="block text-xs font-extrabold uppercase tracking-[0.06em] text-accent">
              {t("nextTicketLabel")}
            </span>
            <span className="flex items-baseline gap-1.5">
              <span className="font-display text-lg font-extrabold leading-tight">
                {t("ticketNumber", { number: nextTicket.ticket.number })}
              </span>
              <span className="truncate text-xs text-muted-foreground">
                {t("ticketQuestionsShort", { count: nextTicket.totalQuestions })}
              </span>
            </span>
          </span>
          <span className="btn-3d-primary inline-flex min-h-11 shrink-0 items-center rounded-xl px-3.5 font-display text-sm font-extrabold">
            {t("solve")}
          </span>
        </button>
      )}

      <div
        className="chip-scroll max-md:grid max-md:grid-cols-2 max-md:gap-1.5 max-md:overflow-visible max-md:pb-0"
        role="group"
        aria-label={t("title")}
      >
        {filterTabs.map((tab) => {
          const isSelected = filterStatus === tab.key;
          return (
            <button
              type="button"
              key={tab.key}
              onClick={() => setFilterStatus(tab.key)}
              aria-pressed={isSelected}
              className={`filter-chip relative transition-colors ${
                isSelected ? "font-extrabold text-accent-foreground" : "filter-chip-idle font-bold"
              }`}
            >
              {isSelected && (
                <span
                  aria-hidden="true"
                  className="nav-pill-in absolute inset-0 rounded-xl bg-accent shadow-3d"
                />
              )}
              <span className="relative z-10">{tab.label}</span>
              {/* Phone only: the wide layout already states these counts in its
                  hero card, and its chips must not move. */}
              <span className="relative z-10 ml-1.5 tabular-nums opacity-70 md:hidden">{tab.count}</span>
            </button>
          );
        })}
      </div>


      {error && (
        <div role="alert" className="rounded-2xl border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive font-medium">
          <p>{t("loadError")}</p>
          <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => void refetch()}>
            <RefreshCw aria-hidden="true" className="mr-2 h-4 w-4" /> {t("retry")}
          </Button>
        </div>
      )}

      {lockNotice && (
        <div
          role="status"
          className="rounded-2xl border border-accent/40 bg-accent/10 p-4 text-sm font-medium text-foreground"
        >
          {lockNotice}
        </div>
      )}

      {/* Tickets grid */}
      {loading ? (
        <div role="status" className="py-12 text-center text-sm text-muted-foreground animate-pulse">{t("loading")}</div>
      ) : filteredTickets.length === 0 ? (
        <div className="py-12 text-center text-sm text-muted-foreground">{t("emptyFiltered")}</div>
      ) : (
        <div className="grid grid-cols-2 gap-3 max-md:grid-cols-5 max-md:gap-2 sm:grid-cols-3 sm:gap-3.5 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          {filteredTickets.map((ticket) => {
            const { bestCorrect, isCompleted, isLocked, attempts, totalQuestions, progressPercent } =
              getTicketState(ticket);

            return (
              <div
                key={ticket.number}
                onClick={() => handleStartTicket(ticket)}
                onKeyDown={(event) => handleTicketKeyDown(event, ticket)}
                role="button"
                tabIndex={0}
                aria-label={t("openTicket", { number: ticket.number })}
                className={ticketTileClass(isCompleted, isLocked, attempts)}
              >
                <div className="flex flex-col items-center justify-center gap-0.5 md:hidden">
                  <span
                    className={`font-display text-lg font-extrabold leading-none tabular-nums ${
                      isCompleted ? "text-gold" : isLocked ? "text-muted-foreground" : "text-foreground"
                    }`}
                  >
                    {ticket.number}
                  </span>
                  {isLocked ? (
                    <Lock aria-hidden="true" className="h-3.5 w-3.5 text-muted-foreground" />
                  ) : (
                    <span
                      className={`whitespace-nowrap text-xs font-bold leading-none tabular-nums ${
                        isCompleted ? "text-gold" : attempts > 0 ? "text-accent" : "text-muted-foreground"
                      }`}
                    >
                      {attempts > 0 || isCompleted
                        ? t("scoreShort", { score: bestCorrect, total: totalQuestions })
                        : t("ticketQuestionsShort", { count: totalQuestions })}
                    </span>
                  )}
                </div>

                <div className="flex w-full items-center justify-between gap-2 pl-1 max-md:hidden">
                  <span className="text-xs font-bold uppercase tracking-[0.12em] text-muted-foreground">
                    {t("ticketNumber", { number: ticket.number })}
                  </span>
                  {isLocked ? (
                    <Lock aria-hidden="true" className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  ) : isCompleted ? (
                    <Star aria-hidden="true" className="h-3.5 w-3.5 shrink-0 fill-gold text-gold" />
                  ) : attempts > 0 ? (
                    <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-accent" aria-hidden="true" />
                  ) : null}
                </div>

                <div className="flex flex-1 flex-col items-center justify-center gap-1 py-2 text-center max-md:hidden">
                  <span
                    className={`ticket-tile-number ${
                      isCompleted ? "text-gold" : isLocked ? "text-muted-foreground" : "text-foreground"
                    }`}
                  >
                    {ticket.number}
                  </span>
                  {!isLocked && attempts > 0 && !isCompleted && (
                    <div className="mt-1 w-full max-w-[7.5rem] space-y-1.5 px-1">
                      <p className="text-xs font-extrabold tabular-nums text-accent">
                        {t("scoreShort", { score: bestCorrect, total: totalQuestions })}
                      </p>
                      <div className="ticket-progress-track">
                        <div
                          className="ticket-progress-fill"
                          style={{ width: `${Math.max(progressPercent, bestCorrect > 0 ? 8 : 0)}%` }}
                        />
                      </div>
                    </div>
                  )}
                  {isCompleted && (
                    <div className="mt-1 w-full max-w-[7.5rem] space-y-1.5 px-1">
                      <p className="inline-flex items-center justify-center gap-1 text-xs font-extrabold tabular-nums text-gold">
                        <Check aria-hidden="true" className="h-3.5 w-3.5" />
                        {t("scoreShort", { score: bestCorrect, total: totalQuestions })}
                      </p>
                      <div className="ticket-progress-track">
                        <div className="ticket-progress-fill ticket-progress-fill-done" style={{ width: "100%" }} />
                      </div>
                    </div>
                  )}
                </div>

                <div className="w-full pl-1 max-md:hidden">
                  {isLocked ? (
                    <span className="inline-flex h-10 w-full items-center justify-center gap-1.5 rounded-xl border border-border bg-muted/40 px-2 text-xs font-bold text-muted-foreground">
                      <Lock aria-hidden="true" className="h-3.5 w-3.5" />
                      {ticket.lock_reason === "prev_required"
                        ? t("lockedPrevRequiredShort")
                        : t("vipRequiredShort")}
                    </span>
                  ) : isCompleted ? (
                    <span className="inline-flex h-10 w-full items-center justify-center gap-1.5 rounded-xl border border-gold/30 bg-gold/10 px-2 text-xs font-extrabold text-gold">
                      <Star aria-hidden="true" className="h-3.5 w-3.5 fill-gold" />
                      {t("filterCompleted")}
                    </span>
                  ) : (
                    <Button as="span" variant="game" size="sm" className="ticket-tile-cta !min-h-9 !border-b-0 !shadow-none">
                      <Play aria-hidden="true" className="h-3 w-3" /> {t("solve")}
                    </Button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </main>
  );
}
