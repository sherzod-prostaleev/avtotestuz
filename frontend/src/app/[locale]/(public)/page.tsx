"use client";

import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { DemoQuestionBlock } from "./demo-question-block";
import { ThemeToggle } from "@/components/theme-toggle";
import {
  Sparkles,
  Award,
  BrainCircuit,
  ShieldCheck,
  Zap,
  HelpCircle,
  ChevronRight,
  LogIn,
  CarFront,
} from "lucide-react";

const gameLinkBase = "inline-flex items-center justify-center rounded-2xl border-b-4 border-accent-shadow bg-accent font-bold tracking-wide text-accent-foreground shadow-3d transition-all duration-150 hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:translate-y-1 active:shadow-none";
const outlineLinkBase = "inline-flex items-center justify-center rounded-2xl border border-border bg-card font-bold tracking-wide text-foreground shadow-sm transition-all duration-150 hover:border-accent hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

export default function LandingPage() {
  const t = useTranslations("Landing");
  const locale = useLocale();

  const features = [
    {
      icon: BrainCircuit,
      color: "text-violet-500",
      bg: "bg-violet-500/10",
      title: t("feature1Title"),
      text: t("feature1Text"),
    },
    {
      icon: Award,
      color: "text-amber-500",
      bg: "bg-amber-500/10",
      title: t("feature4Title"),
      text: t("feature4Text"),
    },
    {
      icon: ShieldCheck,
      color: "text-emerald-500",
      bg: "bg-emerald-500/10",
      title: t("feature2Title"),
      text: t("feature2Text"),
    },
    {
      icon: Zap,
      color: "text-sky-500",
      bg: "bg-sky-500/10",
      title: t("feature3Title"),
      text: t("feature3Text"),
    },
  ];

  const steps = [
    {
      n: "1",
      title: t("step1Title"),
      text: t("step1Text"),
    },
    {
      n: "2",
      title: t("step2Title"),
      text: t("step2Text"),
    },
    {
      n: "3",
      title: t("step3Title"),
      text: t("step3Text"),
    },
  ];

  const faqs = [
    { q: t("faq1Q"), a: t("faq1A") },
    { q: t("faq2Q"), a: t("faq2A") },
    { q: t("faq3Q"), a: t("faq3A") },
    { q: t("faq4Q"), a: t("faq4A") },
  ];

  const proofStats = [
    { value: 1235, label: t("proofQuestions") },
    { value: 61, label: t("proofTickets") },
    { value: 13, label: t("proofTopics") },
    { value: 3, label: t("proofLanguages") },
  ];

  const holdPoints = [
    { title: t("hold1Title"), text: t("hold1Text") },
    { title: t("hold2Title"), text: t("hold2Text") },
    { title: t("hold3Title"), text: t("hold3Text") },
  ];

  const methodSteps = [
    { title: t("methodRecallTitle"), text: t("methodRecallText") },
    { title: t("methodSpacingTitle"), text: t("methodSpacingText") },
    { title: t("methodFeedbackTitle"), text: t("methodFeedbackText") },
  ];

  return (
    <div className="min-h-screen bg-background text-foreground flex flex-col">
      {/* ───── Top Navigation Bar ───── */}
      <header className="sticky top-0 z-40 w-full border-b border-border/60 bg-background/80 backdrop-blur-xl">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-4">
          <Link href={`/${locale}`} className="flex items-center gap-2.5 font-display text-xl font-black tracking-tight">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-accent text-white shadow-3d transition-transform hover:scale-105">
              <CarFront aria-hidden="true" className="h-5 w-5" />
            </div>
            <span className="bg-gradient-to-r from-accent via-indigo-500 to-accent bg-clip-text text-transparent">
              {t("brandName")}
            </span>
          </Link>

          <div className="flex items-center gap-3">
            <ThemeToggle />
            <Link href={`/${locale}/login`} className={`${gameLinkBase} h-9 px-5 text-xs`}>
                <LogIn aria-hidden="true" className="mr-1.5 h-3.5 w-3.5" /> {t("login")}
            </Link>
          </div>
        </div>
      </header>

      {/* ───── HERO SECTION ───── */}
      <section className="relative overflow-hidden border-b border-border/40">
        {/* Subtle decorative gradient blobs */}
        <div className="pointer-events-none absolute -top-32 -left-32 h-96 w-96 rounded-full bg-accent/10 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-32 -right-32 h-96 w-96 rounded-full bg-gold/10 blur-3xl" />

        <div className="relative mx-auto max-w-6xl px-4 py-16 md:py-24 text-center space-y-8">
          {/* Year Badge */}
          <div className="inline-flex items-center gap-2 rounded-full border border-gold/40 bg-gold/10 px-5 py-2 text-xs font-extrabold text-gold shadow-sm animate-fade-in">
            <Sparkles aria-hidden="true" className="h-4 w-4" /> {t("updatedBadge")}
          </div>

          {/* Main Title */}
          <h1 className="mx-auto max-w-4xl font-display text-4xl sm:text-5xl md:text-6xl font-black leading-[1.1] tracking-tight animate-fade-in">
            {t("heroTitleBefore")}{" "}
            <span className="bg-gradient-to-r from-accent via-indigo-500 to-accent bg-clip-text text-transparent">
              {t("heroTitleAccent")}
            </span>{" "}
            {t("heroTitleAfter")}
          </h1>

          {/* Subtitle */}
          <p className="mx-auto max-w-2xl text-base sm:text-lg text-muted-foreground leading-relaxed animate-fade-in">
            {t("heroSubtitle")}
          </p>

          {/* CTA Buttons */}
          <div className="flex flex-col items-center justify-center gap-4 sm:flex-row animate-fade-in">
            <Link href={`/${locale}/login`} className={`${gameLinkBase} h-13 px-10 py-4 text-base font-extrabold shadow-xl`}>
                {t("ctaStart")} <ChevronRight aria-hidden="true" className="ml-2 h-5 w-5" />
            </Link>
            <Link href={`/${locale}/signs`} className={`${outlineLinkBase} h-13 px-8 py-4 text-sm`}>
                {t("ctaSigns")}
            </Link>
          </div>

          <div className="mx-auto grid max-w-4xl gap-3 md:grid-cols-3">
            {holdPoints.map((point) => (
              <div
                key={point.title}
                className="rounded-2xl border border-border/70 bg-card/80 p-4 text-left shadow-sm backdrop-blur"
              >
                <p className="font-display text-sm font-bold">{point.title}</p>
                <p className="mt-1 text-xs leading-relaxed text-muted-foreground">{point.text}</p>
              </div>
            ))}
          </div>

          {/* Proof Stats Row */}
          <div className="mx-auto max-w-3xl pt-8 grid grid-cols-2 gap-4 sm:grid-cols-4">
            {proofStats.map((s) => (
              <div key={s.label} className="rounded-2xl border border-border/60 bg-card/60 backdrop-blur-sm p-4 text-center">
                <p className="font-display text-3xl font-black text-accent">{s.value}</p>
                <p className="mt-1 text-[11px] font-bold text-muted-foreground uppercase tracking-wider">{s.label}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <main className="flex-1 mx-auto max-w-6xl px-4 py-16 space-y-20">
        {/* ───── STUDY METHOD ───── */}
        <section className="space-y-8">
          <div className="text-center space-y-3">
            <span className="inline-block rounded-full bg-accent/10 px-4 py-1.5 text-xs font-extrabold text-accent">
              {t("methodBadge")}
            </span>
            <h2 className="font-display text-2xl sm:text-3xl font-extrabold tracking-tight">{t("methodTitle")}</h2>
            <p className="mx-auto max-w-lg text-sm text-muted-foreground">
              {t("methodSubtitle")}
            </p>
          </div>
          <div className="grid gap-6 md:grid-cols-3">
            {methodSteps.map((item) => (
              <div key={item.title} className="glass-card p-6 space-y-3">
                <p className="font-display text-lg font-bold">{item.title}</p>
                <p className="text-sm leading-relaxed text-muted-foreground">{item.text}</p>
              </div>
            ))}
          </div>
        </section>

        {/* ───── DEMO SECTION ───── */}
        <section className="space-y-8">
          <div className="text-center space-y-3">
            <span className="inline-block rounded-full bg-accent/10 px-4 py-1.5 text-xs font-extrabold text-accent">
              {t("demoBadge")}
            </span>
            <h2 className="font-display text-2xl sm:text-3xl font-extrabold tracking-tight">{t("demoSectionTitle")}</h2>
            <p className="mx-auto max-w-lg text-sm text-muted-foreground">
              {t("demoDescription")}
            </p>
          </div>
          <div className="mx-auto max-w-xl">
            <DemoQuestionBlock />
          </div>
        </section>

        {/* ───── WHY US FEATURES ───── */}
        <section className="space-y-8">
          <div className="text-center space-y-3">
            <h2 className="font-display text-2xl sm:text-3xl font-extrabold tracking-tight">{t("whyUsTitle")}</h2>
            <p className="mx-auto max-w-lg text-sm text-muted-foreground">
              {t("whyUsSubtitle")}
            </p>
          </div>
          <div className="grid gap-6 sm:grid-cols-2">
            {features.map((f) => {
              const Icon = f.icon;
              return (
                <div key={f.title} className="glass-card p-6 flex items-start gap-5 hover:border-accent/50 hover:shadow-xl hover:-translate-y-0.5">
                  <div className={`flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl ${f.bg} ${f.color} shadow-sm`}>
                    <Icon aria-hidden="true" className="h-6 w-6" />
                  </div>
                  <div className="space-y-1.5">
                    <h3 className="font-display text-base font-bold">{f.title}</h3>
                    <p className="text-xs leading-relaxed text-muted-foreground">{f.text}</p>
                  </div>
                </div>
              );
            })}
          </div>
        </section>

        {/* ───── HOW IT WORKS ───── */}
        <section className="space-y-8">
          <div className="text-center space-y-3">
            <h2 className="font-display text-2xl sm:text-3xl font-extrabold tracking-tight">{t("howItWorksTitle")}</h2>
            <p className="mx-auto max-w-lg text-sm text-muted-foreground">
              {t("howItWorksSubtitle")}
            </p>
          </div>
          <div className="grid gap-6 sm:grid-cols-3">
            {steps.map((s) => {
              return (
                <div key={s.n} className="glass-card p-6 text-center space-y-4 hover:border-accent/50 hover:shadow-xl hover:-translate-y-0.5">
                  <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-accent text-white font-display text-xl font-black shadow-3d">
                    {s.n}
                  </div>
                  <h3 className="font-display text-base font-bold">{s.title}</h3>
                  <p className="text-xs leading-relaxed text-muted-foreground">{s.text}</p>
                </div>
              );
            })}
          </div>
        </section>

        {/* ───── FAQ ───── */}
        <section className="space-y-8">
          <div className="text-center space-y-3">
            <h2 className="font-display text-2xl sm:text-3xl font-extrabold tracking-tight">{t("faqTitle")}</h2>
          </div>
          <div className="mx-auto max-w-2xl space-y-3">
            {faqs.map((f) => (
              <details key={f.q} className="glass-card group rounded-2xl p-5 [&_summary::-webkit-details-marker]:hidden">
                <summary className="flex cursor-pointer items-center justify-between text-sm font-bold text-foreground list-none">
                  <span className="flex items-center gap-2.5">
                    <HelpCircle aria-hidden="true" className="h-4 w-4 text-accent shrink-0" /> {f.q}
                  </span>
                  <ChevronRight aria-hidden="true" className="h-4 w-4 text-muted-foreground transition-transform group-open:rotate-90 shrink-0" />
                </summary>
                <p className="mt-3 pt-3 border-t border-border text-xs leading-relaxed text-muted-foreground">{f.a}</p>
              </details>
            ))}
          </div>
        </section>

        {/* ───── BOTTOM CTA ───── */}
        <section className="rounded-3xl border border-accent/30 bg-gradient-to-br from-card via-card to-accent/10 p-10 text-center space-y-5 shadow-xl">
          <h2 className="font-display text-2xl sm:text-3xl font-extrabold">{t("bottomCtaTitle")}</h2>
          <p className="mx-auto max-w-md text-sm text-muted-foreground">
            {t("bottomCtaText")}
          </p>
          <Link href={`/${locale}/login`} className={`${gameLinkBase} h-13 px-10 py-4 text-base font-extrabold shadow-xl`}>
              {t("ctaStart")} <ChevronRight aria-hidden="true" className="ml-2 h-5 w-5" />
          </Link>
        </section>
      </main>

      {/* ───── FOOTER ───── */}
      <footer className="border-t border-border bg-card/50 py-8">
        <div className="mx-auto max-w-6xl px-4 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-muted-foreground">
          <div className="flex items-center gap-2 font-display font-extrabold text-foreground">
            <CarFront aria-hidden="true" className="h-4 w-4" />
            {t("footer", { year: new Date().getFullYear() })}
          </div>
          <p>{t("footerDescription")}</p>
        </div>
      </footer>
    </div>
  );
}
