"use client";

import { useState, type KeyboardEvent } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTickets, TicketItem } from "@/hooks/use-tickets";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
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
    <main className="mx-auto max-w-6xl px-4 py-8 space-y-8">
      <header className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <Link href={`/${locale}/dashboard`} className="mb-2 inline-flex items-center gap-1 text-sm text-accent hover:underline">
            <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("backHome")}
          </Link>
          <h1 className="font-display text-2xl font-bold">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>

        {/* Search Input */}
        <div className="relative w-full sm:w-64">
          <Search aria-hidden="true" className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("searchPlaceholder")}
            aria-label={t("searchLabel")}
            className="w-full rounded-2xl border border-border bg-card py-2.5 pl-9 pr-4 text-xs font-semibold outline-none focus:border-accent"
          />
        </div>
      </header>

      <section className="grid gap-4 lg:grid-cols-[1.15fr_0.85fr]">
        <Card className="glass-card overflow-hidden border-accent/20 bg-gradient-to-br from-card via-card to-accent/10 p-6 md:p-8">
          <div className="flex flex-col gap-6">
            <div className="flex flex-wrap items-center gap-2">
              <span className="rounded-full bg-accent/15 px-3 py-1 text-[11px] font-extrabold text-accent">
                {t("statTotal", { count: tickets.length })}
              </span>
              <span className="rounded-full bg-gold/15 px-3 py-1 text-[11px] font-extrabold text-gold">
                {t("statCompleted", { count: completedCount })}
              </span>
              <span className="rounded-full bg-emerald-500/15 px-3 py-1 text-[11px] font-extrabold text-emerald-600">
                {t("statInProgress", { count: inProgressCount })}
              </span>
              <span className="rounded-full bg-muted px-3 py-1 text-[11px] font-extrabold text-muted-foreground">
                {t("statLocked", { count: lockedCount })}
              </span>
            </div>

            <div className="space-y-3">
              <div className="inline-flex items-center rounded-full border border-border/60 bg-background/80 px-3 py-1 text-[11px] font-bold text-muted-foreground">
                {t("ticketsProgress", { done: completedCount, total: tickets.length })}
              </div>
              <h2 className="font-display text-xl font-extrabold tracking-tight md:text-2xl">
                {t("heroTitle")}
              </h2>
              <p className="max-w-xl text-sm text-muted-foreground">
                {t("heroDescription")}
              </p>
              <div
                className="h-2 w-full overflow-hidden rounded-full border border-border bg-background"
                role="progressbar"
                aria-valuenow={completedPercent}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-label={t("ticketsProgress", { done: completedCount, total: tickets.length })}
              >
                <div
                  className="h-full rounded-full bg-gradient-to-r from-accent to-gold transition-all duration-500"
                  style={{ width: `${completedPercent}%` }}
                />
              </div>
            </div>
          </div>
        </Card>

        <Card className="glass-card flex flex-col justify-between border-border/70 bg-background/80 p-6 md:p-8">
          <div className="flex items-start justify-between gap-4">
            <div>
              <p className="text-[11px] font-bold uppercase tracking-[0.2em] text-muted-foreground">
                {t("nextTicketLabel")}
              </p>
              <h3 className="mt-2 font-display text-3xl font-black tracking-tight">
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
            </div>
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-accent/15 text-accent">
              <Play aria-hidden="true" className="h-6 w-6" />
            </div>
          </div>

          <div className="mt-5 flex flex-wrap gap-2 text-[11px] font-bold">
            <span className="rounded-full bg-card px-3 py-1 text-muted-foreground">
              {t("statAvailable", { count: availableCount })}
            </span>
            <span className="rounded-full bg-card px-3 py-1 text-muted-foreground">
              {t("statUnstarted", { count: unstartedCount })}
            </span>
          </div>

          {nextTicket ? (
            <Button
              type="button"
              variant="game"
              size="lg"
              className="mt-5 w-full"
              onClick={() => handleStartTicket(nextTicket.ticket)}
            >
              {t("solve")}
            </Button>
          ) : (
            <Button type="button" variant="outline" size="lg" className="mt-5 w-full" onClick={() => router.push(`/${locale}/practice`)}>
              {t("goToPractice")}
            </Button>
          )}
        </Card>
      </section>

      {/* Filter Tabs */}
      <div className="flex flex-wrap gap-2">
        {filterTabs.map((tab) => (
          <button
            type="button"
            key={tab.key}
            onClick={() => setFilterStatus(tab.key)}
            aria-pressed={filterStatus === tab.key}
            className={`rounded-xl px-4 py-2 text-xs font-bold transition-all ${
              filterStatus === tab.key
                ? "bg-accent text-accent-foreground shadow-3d"
                : "border border-border bg-card text-muted-foreground hover:text-foreground"
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
                className={`glass-card relative flex flex-col items-center justify-between p-4 text-center cursor-pointer transition-all duration-200 hover:-translate-y-1 hover:shadow-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${
                  isCompleted
                    ? "border-gold/40 bg-gold/5"
                    : isLocked
                    ? "border-border opacity-70 bg-card/50"
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
                    <span className="inline-flex h-7 w-full items-center justify-center rounded-xl border-b-4 border-accent-shadow bg-accent px-3 py-1 text-[11px] font-bold text-accent-foreground shadow-3d">
                      <Play aria-hidden="true" className="mr-1 h-3 w-3" /> {t("solve")}
                    </span>
                  )}
                </div>
              </Card>
            );
          })}
        </div>
      )}
    </main>
  );
}
