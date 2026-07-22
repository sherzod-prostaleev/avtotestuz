"use client";

import { useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTickets, TicketItem } from "@/hooks/use-tickets";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ArrowLeft, Search, Lock, Star, CheckCircle2, Play } from "lucide-react";

export default function TicketsPage() {
  const t = useTranslations("Tickets");
  const locale = useLocale();
  const router = useRouter();
  const { tickets, loading, error } = useTickets();

  const [search, setSearch] = useState("");
  const [filterStatus, setFilterStatus] = useState<"all" | "completed" | "in_progress" | "locked">("all");

  const filteredTickets = tickets.filter((ticket) => {
    const matchesSearch = search === "" || ticket.number.toString().includes(search);
    if (!matchesSearch) return false;
    const bestCorrect = ticket.best_correct ?? ticket.score ?? 0;
    const isCompleted = ticket.status === "completed" || bestCorrect >= 18;
    const isLocked = ticket.status === "locked" || ticket.unlocked === false;
    const attempts = ticket.attempts ?? (ticket.status !== "unstarted" ? 1 : 0);

    if (filterStatus === "completed") return isCompleted;
    if (filterStatus === "in_progress") return attempts > 0 && !isCompleted;
    if (filterStatus === "locked") return isLocked;
    return true;
  });

  const handleStartTicket = (ticket: TicketItem) => {
    const isLocked = ticket.status === "locked" || ticket.unlocked === false;
    if (isLocked) {
      router.push(`/${locale}/premium`);
      return;
    }
    router.push(`/${locale}/session/start?mode=variant&variant_id=${ticket.number}`);
  };

  return (
    <main className="mx-auto max-w-6xl px-4 py-8 space-y-6">
      <header className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <Link href={`/${locale}/dashboard`} className="mb-2 inline-flex items-center gap-1 text-sm text-accent hover:underline">
            <ArrowLeft className="h-4 w-4" /> Bosh sahifaga qaytish
          </Link>
          <h1 className="font-display text-2xl font-bold">{t("title")}</h1>
          <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
        </div>

        {/* Search Input */}
        <div className="relative w-full sm:w-64">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("searchPlaceholder")}
            className="w-full rounded-2xl border border-border bg-card py-2.5 pl-9 pr-4 text-xs font-semibold outline-none focus:border-accent"
          />
        </div>
      </header>

      {/* Filter Tabs */}
      <div className="flex flex-wrap gap-2">
        {[
          { key: "all", label: t("filterAll") },
          { key: "completed", label: t("filterCompleted") },
          { key: "in_progress", label: t("filterInProgress") },
          { key: "locked", label: t("lockedText") },
        ].map((tab) => (
          <button
            key={tab.key}
            onClick={() => setFilterStatus(tab.key as any)}
            className={`rounded-xl px-4 py-2 text-xs font-bold transition-all ${
              filterStatus === tab.key
                ? "bg-accent text-white shadow-3d"
                : "border border-border bg-card text-muted-foreground hover:text-foreground"
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {error && (
        <div className="rounded-2xl border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive font-medium">
          {error}
        </div>
      )}

      {/* 61 Tickets Grid */}
      {loading ? (
        <div className="py-12 text-center text-sm text-muted-foreground animate-pulse">Biletlar yuklanmoqda...</div>
      ) : (
        <div className="grid gap-4 grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
          {filteredTickets.map((ticket) => {
            const bestCorrect = ticket.best_correct ?? ticket.score ?? 0;
            const isCompleted = ticket.status === "completed" || bestCorrect >= 18;
            const isLocked = ticket.status === "locked" || ticket.unlocked === false;
            const attempts = ticket.attempts ?? (ticket.status !== "unstarted" ? 1 : 0);

            return (
              <Card
                key={ticket.number}
                onClick={() => handleStartTicket(ticket)}
                className={`glass-card relative flex flex-col items-center justify-between p-4 text-center cursor-pointer transition-all duration-200 hover:-translate-y-1 hover:shadow-lg ${
                  isCompleted
                    ? "border-gold/40 bg-gold/5"
                    : isLocked
                    ? "border-border opacity-70 bg-card/50"
                    : "border-accent/30 hover:border-accent"
                }`}
              >
                {/* Top Badge */}
                <div className="w-full flex items-center justify-between text-[11px] font-extrabold text-muted-foreground">
                  <span>Bilet {ticket.number}</span>
                  {isLocked ? (
                    <Lock className="h-3.5 w-3.5 text-muted-foreground" />
                  ) : isCompleted ? (
                    <Star className="h-3.5 w-3.5 fill-gold text-gold" />
                  ) : null}
                </div>

                {/* Main Ticket Icon Number */}
                <div className="my-3 flex h-14 w-14 items-center justify-center rounded-2xl bg-card border border-border font-display text-2xl font-black text-foreground shadow-sm">
                  {ticket.number}
                </div>

                {/* Bottom Status / Score */}
                <div className="w-full">
                  {isLocked ? (
                    <span className="text-[10px] font-bold text-muted-foreground">VIP required</span>
                  ) : isCompleted ? (
                    <span className="inline-flex items-center gap-1 rounded-full bg-gold/20 px-2 py-0.5 text-[11px] font-extrabold text-gold">
                      {bestCorrect}/20
                    </span>
                  ) : attempts > 0 ? (
                    <span className="text-[11px] font-bold text-accent">
                      {bestCorrect}/20
                    </span>
                  ) : (
                    <Button variant="game" size="sm" className="w-full py-1 text-[11px] h-7">
                      <Play className="mr-1 h-3 w-3" /> Yechish
                    </Button>
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
