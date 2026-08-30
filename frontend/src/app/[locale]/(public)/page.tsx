"use client";

import { useEffect, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { DemoQuestionBlock } from "./demo-question-block";
import { BrandLogo } from "@/components/brand/brand-logo";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { ThemeToggle } from "@/components/theme-toggle";
import { Reveal } from "@/components/shared/reveal";
import { useCatalogCounts } from "@/hooks/use-variant-count";
import { apiGet } from "@/lib/api-client";
import { contactOrFallback, resolvePhonePair, type SiteContacts } from "@/lib/site-contacts";
import { homeOrFallback, type SiteHomeHero } from "@/lib/site-home";
import {
  Award,
  BrainCircuit,
  ShieldCheck,
  Zap,
  HelpCircle,
  ChevronRight,
  LogIn,
  Phone,
  Mail,
  MapPin,
  Send,
  Instagram,
  Clock3,
} from "lucide-react";

const primaryCta =
  "inline-flex items-center justify-center whitespace-normal rounded-2xl border-b-4 border-accent-shadow bg-accent text-center font-bold tracking-wide text-accent-foreground shadow-3d transition-all duration-150 hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:translate-y-1 active:shadow-none sm:whitespace-nowrap";
const outlineCta =
  "inline-flex items-center justify-center whitespace-normal rounded-2xl border border-border/80 bg-card/80 text-center font-bold tracking-wide text-foreground shadow-raised-sm backdrop-blur-sm transition-[transform,box-shadow,border-color,color] duration-150 hover:-translate-y-0.5 hover:border-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:translate-y-0.5 active:shadow-none sm:whitespace-nowrap";
const heroPrimaryCta = `${primaryCta} h-12 min-h-12 w-full px-5 text-sm font-extrabold sm:w-auto sm:px-8 sm:text-[15px]`;
const heroOutlineCta = `${outlineCta} h-12 min-h-12 w-full px-5 text-sm sm:w-auto sm:px-6`;

/** Dominant hero visual — official exam cockpit, not a floating media card. */
function ExamCockpit({
  timerLabel,
  questionLabel,
  optionA,
  optionB,
}: {
  timerLabel: string;
  questionLabel: string;
  optionA: string;
  optionB: string;
}) {
  return (
    <div
      aria-hidden="true"
      className="relative w-full max-w-lg justify-self-stretch md:max-w-none md:justify-self-end"
    >
      <div className="landing-cockpit relative overflow-hidden rounded-2xl border border-white/10 bg-[#0a1624]">
        <div className="landing-cockpit-sweep pointer-events-none absolute inset-y-0 left-0 w-1/3 bg-gradient-to-r from-accent/20 to-transparent" />
        <div className="flex items-center justify-between border-b border-[#1c3554] bg-[#081320] px-4 py-2.5">
          <span className="font-display text-sm font-black tracking-wide text-white">
            Driver <span className="text-accent">Go</span>
          </span>
          <span className="rounded-md border border-amber-300/30 bg-amber-300/10 px-2 py-0.5 font-mono text-sm font-bold tabular-nums text-amber-300 shadow-[0_2px_0_0_hsl(38_85%_28%)]">
            {timerLabel}
          </span>
        </div>
        <div className="border-b border-[#204a75] bg-[#0d2e4d] px-4 py-3 text-center text-sm font-extrabold leading-snug text-white">
          {questionLabel}
        </div>
        <div className="grid gap-2 p-4 sm:grid-cols-[1.1fr_1fr] sm:gap-3">
          <div className="space-y-2">
            {[optionA, optionB].map((text, index) => (
              <div
                key={text}
                className={`flex overflow-hidden rounded-xl border text-left text-xs font-semibold text-white shadow-[0_2px_0_0_rgba(0,0,0,0.45)] ${
                  index === 0
                    ? "border-green-500/80 bg-[#163820]"
                    : "border-[#354f6e] bg-[#162738]"
                }`}
              >
                <span
                  className={`flex w-9 shrink-0 items-center justify-center border-r text-[11px] font-black ${
                    index === 0
                      ? "border-green-500 bg-green-600"
                      : "border-[#354f6e] bg-[#1d334a]"
                  }`}
                >
                  F{index + 1}
                </span>
                <span className="px-3 py-2.5 leading-snug">{text}</span>
              </div>
            ))}
          </div>
          <div className="relative min-h-[7.5rem] overflow-hidden rounded-xl border border-slate-500/40 bg-black shadow-[inset_0_0_0_1px_rgba(255,255,255,0.04),0_2px_0_0_rgba(0,0,0,0.5)]">
            <div className="absolute inset-0 opacity-40">
              <div className="landing-hero-lane !opacity-70" />
            </div>
            <div className="absolute right-3 top-3 flex flex-col gap-1.5">
              <span className="landing-signal h-2.5 w-2.5 rounded-full bg-accent" />
              <span className="h-2.5 w-2.5 rounded-full bg-slate-600" />
              <span className="h-2.5 w-2.5 rounded-full bg-slate-600" />
            </div>
          </div>
        </div>
        <div className="chip-scroll gap-1.5 border-t border-[#1c3554] bg-[#081320] px-3 py-3 sm:gap-1.5 sm:px-4">
          {Array.from({ length: 8 }, (_, i) => (
            <span
              key={i}
              className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-[11px] font-extrabold shadow-[0_2px_0_0_rgba(0,0,0,0.55)] ${
                i === 0
                  ? "bg-green-600 text-white ring-2 ring-white/80"
                  : i === 1
                    ? "bg-blue-600 text-white"
                    : "bg-[#1c334d] text-slate-200"
              }`}
            >
              {i + 1}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}

export default function LandingPage() {
  const t = useTranslations("Landing");
  const locale = useLocale();
  const { tickets: ticketCount, questions: questionCount, topics: topicCount } = useCatalogCounts();
  const [contacts, setContacts] = useState<SiteContacts | null>(null);
  const [home, setHome] = useState<SiteHomeHero | null>(null);

  useEffect(() => {
    let cancelled = false;
    void apiGet<SiteContacts>("site/contacts")
      .then((data) => {
        if (!cancelled) setContacts(data);
      })
      .catch(() => {
        /* keep i18n placeholders */
      });
    void apiGet<SiteHomeHero>("site/home")
      .then((data) => {
        if (!cancelled) setHome(data);
      })
      .catch(() => {
        /* keep i18n placeholders */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const { phone, phoneTel } = resolvePhonePair(
    contacts?.phone,
    contacts?.phoneTel,
    t("footerPhone"),
    t("footerPhoneTel"),
  );
  const email = contactOrFallback(contacts?.email, t("footerEmail"));
  const address = contactOrFallback(contacts?.address, t("footerAddress"));
  const hours = contactOrFallback(contacts?.hours, t("footerHours"));
  const telegram = contactOrFallback(contacts?.telegram, t("footerTelegram"));
  const telegramUrl = contactOrFallback(contacts?.telegramUrl, t("footerTelegramUrl"));
  const instagram = contactOrFallback(contacts?.instagram, t("footerInstagram"));
  const instagramUrl = contactOrFallback(contacts?.instagramUrl, t("footerInstagramUrl"));

  const i18nHeadline = `${t("heroTitleBefore")} ${t("heroTitleAccent")} ${t("heroTitleAfter")}`;
  const heroHeadline = homeOrFallback(home?.headline, i18nHeadline);
  const heroSubtitle = homeOrFallback(home?.subtitle, t("heroSubtitle"));
  const heroCtaLabel = homeOrFallback(home?.ctaLabel, t("ctaStart"));
  // Product invariant: the hero promises a free diagnostic, so its target
  // must never be redirected to login by stale CMS data.
  const heroCtaHref = `/${locale}/diagnostic`;
  const useCmsHeadline = Boolean(home?.headline?.trim());

  const features = [
    { icon: BrainCircuit, title: t("feature1Title"), text: t("feature1Text") },
    { icon: Award, title: t("feature4Title"), text: t("feature4Text") },
    { icon: ShieldCheck, title: t("feature2Title"), text: t("feature2Text") },
    { icon: Zap, title: t("feature3Title"), text: t("feature3Text") },
  ];

  const steps = [
    { n: "1", title: t("step1Title"), text: t("step1Text") },
    { n: "2", title: t("step2Title"), text: t("step2Text") },
    { n: "3", title: t("step3Title"), text: t("step3Text") },
  ];

  const faqs = [
    { q: t("faq1Q"), a: t("faq1A") },
    { q: t("faq2Q"), a: t("faq2A") },
    { q: t("faq3Q"), a: t("faq3A") },
    { q: t("faq4Q"), a: t("faq4A", { topics: topicCount }) },
  ];

  const methodSteps = [
    { title: t("methodRecallTitle"), text: t("methodRecallText") },
    { title: t("methodSpacingTitle"), text: t("methodSpacingText") },
    { title: t("methodFeedbackTitle"), text: t("methodFeedbackText") },
  ];

  const productFacts = [
    { value: t("factQuestionsValue", { questions: questionCount }), label: t("proofQuestions") },
    { value: t("factTicketsValue", { count: ticketCount }), label: t("proofTickets") },
    { value: t("factTopicsValue", { topics: topicCount }), label: t("proofTopics") },
    { value: t("factLanguagesValue"), label: t("proofLanguages") },
  ];

  const holds = [
    { title: t("hold1Title"), text: t("hold1Text") },
    { title: t("hold2Title"), text: t("hold2Text") },
    { title: t("hold3Title"), text: t("hold3Text") },
  ];

  const footerNav = [
    { href: `/${locale}/login`, label: t("login") },
    { href: `/${locale}/narxlar`, label: t("footerNavPricing") },
    { href: `/${locale}/jarimalar`, label: t("footerNavFines") },
    { href: `/${locale}/oferta`, label: t("footerNavOferta") },
    { href: `/${locale}/privacy`, label: t("footerNavPrivacy") },
    { href: `/${locale}/signs`, label: t("ctaSigns") },
    { href: "#demo", label: t("ctaNoSignup") },
    { href: `/${locale}/premium`, label: t("footerNavPremium") },
  ];

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header
        className="sticky top-0 z-40 w-full border-b border-border/60 bg-background/85 shadow-[0_8px_24px_-18px_hsl(var(--elev-ambient)/0.55)] backdrop-blur-md"
        style={{ paddingTop: "env(safe-area-inset-top)" }}
      >
        <div className="mx-auto flex h-14 max-w-6xl items-center justify-between gap-2 px-3 sm:h-16 sm:gap-3 sm:px-4">
          <Link
            prefetch={false}
            href={`/${locale}`}
            className="flex min-w-0 items-center gap-2 font-display text-lg font-black tracking-tight text-foreground sm:gap-2.5 sm:text-2xl"
          >
            <BrandLogo size={36} className="h-8 w-8 shrink-0 rounded-2xl object-cover shadow-raised-sm sm:h-9 sm:w-9" />
            <span className="truncate">{t("brandName")}</span>
          </Link>

          <nav className="hidden items-center gap-2 text-sm font-bold text-muted-foreground md:flex">
            <a
              href="#demo"
              className="rounded-xl border border-border/70 bg-card/60 px-3 py-1.5 shadow-raised-sm transition-[transform,color,border-color] hover:-translate-y-0.5 hover:border-accent/40 hover:text-foreground"
            >
              {t("navTry")}
            </a>
            <a
              href="#method"
              className="rounded-xl border border-border/70 bg-card/60 px-3 py-1.5 shadow-raised-sm transition-[transform,color,border-color] hover:-translate-y-0.5 hover:border-accent/40 hover:text-foreground"
            >
              {t("navMethod")}
            </a>
            <a
              href="#faq"
              className="rounded-xl border border-border/70 bg-card/60 px-3 py-1.5 shadow-raised-sm transition-[transform,color,border-color] hover:-translate-y-0.5 hover:border-accent/40 hover:text-foreground"
            >
              {t("faqTitle")}
            </a>
          </nav>

          <div className="flex shrink-0 items-center gap-1.5 sm:gap-3">
            <LocaleSwitcher compact className="sm:hidden" />
            <LocaleSwitcher className="hidden sm:flex" />
            <ThemeToggle />
            <Link
              prefetch={false}
              href={`/${locale}/login`}
              aria-label={t("login")}
              className={`${primaryCta} h-9 gap-1.5 px-3 text-xs sm:h-9 sm:px-4 sm:text-sm`}
            >
              <LogIn aria-hidden="true" className="h-4 w-4 shrink-0 sm:h-3.5 sm:w-3.5" />
              <span>{t("login")}</span>
            </Link>
          </div>
        </div>
      </header>

      {/* Hero — one composition: brand, headline, line, CTAs, dominant exam stage */}
      <section className="landing-hero-stage relative overflow-hidden border-b border-border">
        <div className="landing-hero-lane" />
        <div className="relative z-10 mx-auto grid max-w-6xl items-center gap-8 px-4 py-10 md:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)] md:gap-12 md:py-20 lg:py-24">
          <div className="min-w-0 space-y-5 text-left animate-fade-in sm:space-y-7">
            {/* Brand already in sticky header — keep hero-level mark from md up. */}
            <p className="hidden font-display text-5xl font-black tracking-tight text-foreground md:block md:text-6xl">
              {t("brandName")}
            </p>
            <p className="text-[11px] font-bold uppercase leading-snug tracking-[0.12em] text-accent sm:text-xs sm:tracking-[0.18em]">
              {t("heroAudience")}
            </p>
            <h1 className="max-w-xl font-display text-[1.85rem] font-black leading-[1.1] tracking-tight sm:text-5xl md:text-[3.35rem]">
              {useCmsHeadline ? (
                heroHeadline
              ) : (
                <>
                  {t("heroTitleBefore")}{" "}
                  <span className="text-accent">{t("heroTitleAccent")}</span>
                  {t("heroTitleAfter") ? <> {t("heroTitleAfter")}</> : null}
                </>
              )}
            </h1>
            <p className="max-w-md text-sm leading-relaxed text-muted-foreground sm:text-lg">
              {heroSubtitle}
            </p>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-3">
              <Link prefetch={false} href={heroCtaHref} className={heroPrimaryCta}>
                {heroCtaLabel}
                <ChevronRight aria-hidden="true" className="ml-1.5 h-4 w-4 shrink-0 sm:ml-2 sm:h-5 sm:w-5" />
              </Link>
              <a href="#demo" className={heroOutlineCta}>
                {t("ctaNoSignup")}
              </a>
            </div>
            <div className="flex items-center gap-2 pt-1 text-xs text-muted-foreground sm:text-sm">
              <span>{t("haveAccountPrompt")}</span>
              <Link
                prefetch={false}
                href={`/${locale}/login`}
                className="font-bold text-accent underline-offset-4 hover:underline"
              >
                {t("login")}
              </Link>
              <span aria-hidden="true">·</span>
              <Link
                prefetch={false}
                href={`/${locale}/register`}
                className="font-bold text-accent underline-offset-4 hover:underline"
              >
                {t("registerCta")}
              </Link>
            </div>
          </div>

          <div className="animate-fade-in" style={{ animationDelay: "140ms" }}>
            <ExamCockpit
              timerLabel={t("cockpitTimer")}
              questionLabel={t("cockpitQuestion")}
              optionA={t("cockpitOptionA")}
              optionB={t("cockpitOptionB")}
            />
          </div>
        </div>
      </section>

      {/* Demo-first conversion magnet — product before more marketing */}
      <section id="demo" className="scroll-mt-24 border-b border-border bg-card/40">
        <div className="mx-auto max-w-6xl space-y-10 px-4 py-16">
          <Reveal className="max-w-2xl space-y-3">
            <p className="text-xs font-extrabold uppercase tracking-[0.16em] text-accent">
              {t("demoBadge")}
            </p>
            <h2 className="font-display text-3xl font-extrabold tracking-tight sm:text-4xl">
              {t("demoSectionTitle")}
            </h2>
            <p className="text-base text-muted-foreground">{t("demoDescription")}</p>
          </Reveal>

          <div className="grid gap-8 lg:grid-cols-[minmax(0,1.15fr)_minmax(0,0.85fr)] lg:items-start">
            <Reveal delayMs={60} className="mx-auto w-full max-w-xl lg:mx-0">
              <div className="landing-panel p-5 sm:p-6">
                <DemoQuestionBlock questionCount={questionCount} />
              </div>
            </Reveal>
            <div className="space-y-4">
              {holds.map((item, i) => (
                <Reveal
                  key={item.title}
                  delayMs={100 + i * 80}
                  className="landing-panel-sm landing-panel-interactive p-4 sm:p-5"
                >
                  <p className="font-display text-lg font-bold">{item.title}</p>
                  <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">{item.text}</p>
                </Reveal>
              ))}
            </div>
          </div>
        </div>
      </section>

      <Reveal className="border-b border-border">
        <div className="mx-auto grid max-w-6xl grid-cols-2 gap-3 px-4 py-10 sm:grid-cols-4 sm:gap-4">
          {productFacts.map((fact, i) => (
            <Reveal key={fact.label} delayMs={i * 70} className="landing-fact text-center sm:text-left">
              <p className="font-display text-3xl font-black tabular-nums text-foreground sm:text-4xl">
                {fact.value}
              </p>
              <p className="mt-1 text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
                {fact.label}
              </p>
            </Reveal>
          ))}
        </div>
      </Reveal>

      <main className="mx-auto w-full max-w-6xl flex-1 space-y-24 px-4 py-20">
        <Reveal>
          <section id="method" className="scroll-mt-24 space-y-10">
            <div className="max-w-xl space-y-3">
              <p className="text-xs font-extrabold uppercase tracking-[0.16em] text-accent">
                {t("methodBadge")}
              </p>
              <h2 className="font-display text-3xl font-extrabold tracking-tight sm:text-4xl">
                {t("methodTitle")}
              </h2>
              <p className="text-base text-muted-foreground">{t("methodSubtitle")}</p>
            </div>
            <div className="grid gap-4 md:grid-cols-3">
              {methodSteps.map((item, i) => (
                <Reveal
                  key={item.title}
                  delayMs={i * 90}
                  className="landing-panel landing-panel-interactive space-y-2 p-5"
                >
                  <p className="font-display text-xl font-bold">{item.title}</p>
                  <p className="text-sm leading-relaxed text-muted-foreground">{item.text}</p>
                </Reveal>
              ))}
            </div>
          </section>
        </Reveal>

        <Reveal>
          <section className="space-y-10">
            <div className="max-w-xl space-y-3">
              <h2 className="font-display text-3xl font-extrabold tracking-tight sm:text-4xl">
                {t("whyUsTitle")}
              </h2>
              <p className="text-base text-muted-foreground">{t("whyUsSubtitle")}</p>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              {features.map((f, i) => {
                const Icon = f.icon;
                return (
                  <Reveal
                    key={f.title}
                    delayMs={i * 80}
                    className="landing-panel landing-panel-interactive flex items-start gap-4 p-5"
                  >
                    <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-accent/15 text-foreground shadow-raised-sm">
                      <Icon aria-hidden="true" className="h-5 w-5" />
                    </div>
                    <div className="space-y-1">
                      <h3 className="font-display text-base font-bold">{f.title}</h3>
                      <p className="text-sm leading-relaxed text-muted-foreground">{f.text}</p>
                    </div>
                  </Reveal>
                );
              })}
            </div>
          </section>
        </Reveal>

        <Reveal>
          <section className="space-y-10">
            <div className="max-w-xl space-y-3">
              <h2 className="font-display text-3xl font-extrabold tracking-tight sm:text-4xl">
                {t("howItWorksTitle")}
              </h2>
              <p className="text-base text-muted-foreground">{t("howItWorksSubtitle")}</p>
            </div>
            <div className="grid gap-4 md:grid-cols-3">
              {steps.map((s, i) => (
                <Reveal key={s.n} delayMs={i * 100} className="landing-panel landing-panel-interactive space-y-3 p-5">
                  <p className="font-display text-4xl font-black text-accent drop-shadow-[0_3px_0_hsl(var(--accent-shadow))] sm:text-5xl">
                    {s.n}
                  </p>
                  <h3 className="font-display text-lg font-bold">{s.title}</h3>
                  <p className="text-sm leading-relaxed text-muted-foreground">{s.text}</p>
                </Reveal>
              ))}
            </div>
          </section>
        </Reveal>

        <Reveal>
          <section id="faq" className="scroll-mt-24 space-y-8">
            <h2 className="font-display text-3xl font-extrabold tracking-tight sm:text-4xl">
              {t("faqTitle")}
            </h2>
            <div className="mx-auto max-w-2xl space-y-3">
              {faqs.map((f) => (
                <details
                  key={f.q}
                  className="landing-panel-sm group px-4 py-4 transition-[border-color,box-shadow] open:border-accent/35 [&_summary::-webkit-details-marker]:hidden"
                >
                  <summary className="flex cursor-pointer list-none items-center justify-between gap-3 text-sm font-bold">
                    <span className="flex items-center gap-2.5">
                      <HelpCircle aria-hidden="true" className="h-4 w-4 shrink-0 text-accent" />
                      {f.q}
                    </span>
                    <ChevronRight
                      aria-hidden="true"
                      className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-90"
                    />
                  </summary>
                  <p className="mt-3 pl-7 text-sm leading-relaxed text-muted-foreground">{f.a}</p>
                </details>
              ))}
            </div>
          </section>
        </Reveal>

        <Reveal>
          <section className="landing-panel relative overflow-hidden px-6 py-12 text-center sm:px-12">
            <div className="pointer-events-none absolute inset-0 opacity-40">
              <div className="landing-hero-lane" />
            </div>
            <div className="relative">
              <h2 className="font-display text-3xl font-extrabold sm:text-4xl">{t("bottomCtaTitle")}</h2>
              <p className="mx-auto mt-3 max-w-md text-base text-muted-foreground">{t("bottomCtaText")}</p>
              <Link prefetch={false} href={heroCtaHref} className={`${heroPrimaryCta} mt-7`}>
                {heroCtaLabel}
                <ChevronRight aria-hidden="true" className="ml-1.5 h-4 w-4 shrink-0 sm:ml-2 sm:h-5 sm:w-5" />
              </Link>
            </div>
          </section>
        </Reveal>
      </main>

      <footer id="contact" className="border-t border-border bg-card">
        <div className="mx-auto grid max-w-6xl gap-10 px-4 py-12 sm:grid-cols-2 lg:grid-cols-4">
          <div className="space-y-3 sm:col-span-2 lg:col-span-1">
            <p className="font-display text-xl font-black text-foreground">{t("brandName")}</p>
            <p className="text-sm leading-relaxed text-muted-foreground">{t("footerDescription")}</p>
            <p className="text-xs text-muted-foreground">{t("footer", { year: new Date().getFullYear() })}</p>
          </div>

          <div className="space-y-3">
            <p className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground">
              {t("footerNavTitle")}
            </p>
            <ul className="space-y-2 text-sm font-semibold">
              {footerNav.map((item) => (
                <li key={item.href}>
                  {item.href.startsWith("#") ? (
                    <a href={item.href} className="text-foreground hover:text-accent">
                      {item.label}
                    </a>
                  ) : (
                    <Link prefetch={false} href={item.href} className="text-foreground hover:text-accent">
                      {item.label}
                    </Link>
                  )}
                </li>
              ))}
            </ul>
          </div>

          <div className="space-y-3">
            <p className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground">
              {t("footerContactTitle")}
            </p>
            <ul className="space-y-3 text-sm">
              <li>
                <a
                  href={`tel:${phoneTel}`}
                  className="inline-flex items-center gap-2 font-semibold text-foreground hover:text-accent"
                >
                  <Phone aria-hidden="true" className="h-4 w-4 text-accent" />
                  {phone}
                </a>
              </li>
              <li>
                <a
                  href={`mailto:${email}`}
                  className="inline-flex items-center gap-2 font-semibold text-foreground hover:text-accent"
                >
                  <Mail aria-hidden="true" className="h-4 w-4 text-accent" />
                  {email}
                </a>
              </li>
              <li className="flex items-start gap-2 text-muted-foreground">
                <MapPin aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0 text-accent" />
                <span>{address}</span>
              </li>
              <li className="flex items-center gap-2 text-muted-foreground">
                <Clock3 aria-hidden="true" className="h-4 w-4 shrink-0 text-accent" />
                <span>{hours}</span>
              </li>
            </ul>
          </div>

          <div className="space-y-3">
            <p className="text-[11px] font-extrabold uppercase tracking-wider text-muted-foreground">
              {t("footerSocialTitle")}
            </p>
            <ul className="space-y-3 text-sm font-semibold">
              <li>
                <a
                  href={telegramUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 text-foreground hover:text-accent"
                >
                  <Send aria-hidden="true" className="h-4 w-4 text-accent" />
                  {telegram}
                </a>
              </li>
              <li>
                <a
                  href={instagramUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 text-foreground hover:text-accent"
                >
                  <Instagram aria-hidden="true" className="h-4 w-4 text-accent" />
                  {instagram}
                </a>
              </li>
            </ul>
            <p className="pt-2 text-xs leading-relaxed text-muted-foreground">{t("footerLegal")}</p>
          </div>
        </div>
      </footer>
    </div>
  );
}
