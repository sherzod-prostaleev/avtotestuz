"use client";

import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { DemoQuestionBlock } from "./demo-question-block";
import { BrandLogo } from "@/components/brand/brand-logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { Reveal } from "@/components/shared/reveal";
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
  "inline-flex items-center justify-center rounded-2xl border-b-4 border-accent-shadow bg-accent font-bold tracking-wide text-accent-foreground shadow-3d transition-all duration-150 hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:translate-y-1 active:shadow-none";
const outlineCta =
  "inline-flex items-center justify-center rounded-2xl border border-border bg-card font-bold tracking-wide text-foreground transition-colors duration-150 hover:border-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

function PhoneMock({
  readiness,
  streak,
  nextAction,
}: {
  readiness: string;
  streak: string;
  nextAction: string;
}) {
  return (
    <div aria-hidden="true" className="relative mx-auto w-[240px] sm:w-[260px]">
      <div className="animate-float rounded-[2rem] border border-border bg-card p-2 shadow-modal">
        <div className="overflow-hidden rounded-[1.5rem] border border-border bg-background">
          <div className="flex items-center justify-between border-b border-border px-4 py-3">
            <span className="font-display text-sm font-black text-foreground">Driver Go</span>
            <span className="rounded-md bg-accent/15 px-2 py-0.5 text-[10px] font-bold text-foreground">
              {streak}
            </span>
          </div>
          <div className="space-y-3 p-4">
            <div>
              <p className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                {readiness}
              </p>
              <p className="mt-1 font-display text-3xl font-black tabular-nums text-foreground">72%</p>
              <div className="mt-2 h-2 overflow-hidden rounded-full bg-border">
                <div className="animate-grow-bar h-full w-[72%] rounded-full bg-success" />
              </div>
            </div>
            <div className="rounded-xl border border-border bg-card p-3">
              <p className="text-[11px] font-bold leading-snug text-foreground">{nextAction}</p>
              <div className="mt-3 h-8 rounded-lg bg-accent text-center text-[11px] font-extrabold leading-8 text-accent-foreground">
                →
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function LandingPage() {
  const t = useTranslations("Landing");
  const locale = useLocale();

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
    { q: t("faq4Q"), a: t("faq4A") },
  ];

  const methodSteps = [
    { title: t("methodRecallTitle"), text: t("methodRecallText") },
    { title: t("methodSpacingTitle"), text: t("methodSpacingText") },
    { title: t("methodFeedbackTitle"), text: t("methodFeedbackText") },
  ];

  const productFacts = [
    { value: t("factQuestionsValue"), label: t("proofQuestions") },
    { value: t("factTicketsValue"), label: t("proofTickets") },
    { value: t("factTopicsValue"), label: t("proofTopics") },
    { value: t("factLanguagesValue"), label: t("proofLanguages") },
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
      <header className="sticky top-0 z-40 w-full border-b border-border/70 bg-background/90 backdrop-blur-md">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-4">
          <Link
            href={`/${locale}`}
            className="flex items-center gap-2.5 font-display text-xl font-black tracking-tight text-foreground sm:text-2xl"
          >
            <BrandLogo size={36} className="h-9 w-9 rounded-2xl object-cover" />
            <span>{t("brandName")}</span>
          </Link>

          <div className="flex items-center gap-3">
            <ThemeToggle />
            <Link href={`/${locale}/login`} className={`${primaryCta} h-9 px-4 text-xs`}>
              <LogIn aria-hidden="true" className="mr-1.5 h-3.5 w-3.5" />
              {t("login")}
            </Link>
          </div>
        </div>
      </header>

      <section className="asphalt-hero asphalt-hero-live relative overflow-hidden border-b border-border">
        <div className="relative mx-auto grid max-w-6xl items-center gap-10 px-4 py-14 md:grid-cols-2 md:gap-12 md:py-20">
          <div className="space-y-6 text-left animate-fade-in">
            <p className="font-display text-3xl font-black tracking-tight text-foreground sm:text-4xl">
              {t("brandName")}
            </p>
            <h1 className="max-w-xl font-display text-4xl font-black leading-[1.08] tracking-tight sm:text-5xl md:text-6xl">
              {t("heroTitleBefore")}{" "}
              <span className="text-accent">{t("heroTitleAccent")}</span>{" "}
              {t("heroTitleAfter")}
            </h1>
            <p className="max-w-md text-base leading-relaxed text-muted-foreground sm:text-lg">
              {t("heroSubtitle")}
            </p>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
              <Link
                href={`/${locale}/login`}
                className={`${primaryCta} h-12 px-8 text-sm font-extrabold sm:text-base`}
              >
                {t("ctaStart")}
                <ChevronRight aria-hidden="true" className="ml-2 h-5 w-5" />
              </Link>
              <a href="#demo" className={`${outlineCta} h-12 px-6 text-sm`}>
                {t("ctaNoSignup")}
              </a>
            </div>
          </div>

          <div className="animate-fade-in md:justify-self-end" style={{ animationDelay: "120ms" }}>
            <PhoneMock
              readiness={t("mockReadiness")}
              streak={t("mockStreak")}
              nextAction={t("mockNextAction")}
            />
          </div>
        </div>
      </section>

      <Reveal className="border-b border-border bg-card/60">
        <div className="mx-auto grid max-w-6xl grid-cols-2 gap-4 px-4 py-8 sm:grid-cols-4 sm:gap-6">
          {productFacts.map((fact, i) => (
            <Reveal key={fact.label} delayMs={i * 70} className="text-center sm:text-left">
              <p className="font-display text-2xl font-black tabular-nums text-foreground sm:text-3xl">
                {fact.value}
              </p>
              <p className="mt-1 text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
                {fact.label}
              </p>
            </Reveal>
          ))}
        </div>
      </Reveal>

      <main className="mx-auto w-full max-w-6xl flex-1 space-y-20 px-4 py-16">
        <Reveal>
          <section className="space-y-8">
            <div className="max-w-xl space-y-3">
              <h2 className="font-display text-2xl font-extrabold tracking-tight sm:text-3xl">
                {t("methodTitle")}
              </h2>
              <p className="text-sm text-muted-foreground">{t("methodSubtitle")}</p>
            </div>
            <div className="grid gap-8 md:grid-cols-3">
              {methodSteps.map((item, i) => (
                <Reveal key={item.title} delayMs={i * 90} className="space-y-2 border-t border-border pt-4">
                  <p className="font-display text-lg font-bold">{item.title}</p>
                  <p className="text-sm leading-relaxed text-muted-foreground">{item.text}</p>
                </Reveal>
              ))}
            </div>
          </section>
        </Reveal>

        <Reveal>
          <section id="demo" className="scroll-mt-24 space-y-8">
            <div className="max-w-xl space-y-3">
              <h2 className="font-display text-2xl font-extrabold tracking-tight sm:text-3xl">
                {t("demoSectionTitle")}
              </h2>
              <p className="text-sm text-muted-foreground">{t("demoDescription")}</p>
            </div>
            <div className="mx-auto max-w-xl">
              <DemoQuestionBlock />
            </div>
          </section>
        </Reveal>

        <Reveal>
          <section className="space-y-8">
            <div className="max-w-xl space-y-3">
              <h2 className="font-display text-2xl font-extrabold tracking-tight sm:text-3xl">
                {t("whyUsTitle")}
              </h2>
              <p className="text-sm text-muted-foreground">{t("whyUsSubtitle")}</p>
            </div>
            <div className="grid gap-6 sm:grid-cols-2">
              {features.map((f, i) => {
                const Icon = f.icon;
                return (
                  <Reveal
                    key={f.title}
                    delayMs={i * 80}
                    className="flex items-start gap-4 border-t border-border pt-5"
                  >
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-accent/15 text-foreground transition-transform duration-300 hover:scale-105">
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
          <section className="space-y-8">
            <div className="max-w-xl space-y-3">
              <h2 className="font-display text-2xl font-extrabold tracking-tight sm:text-3xl">
                {t("howItWorksTitle")}
              </h2>
              <p className="text-sm text-muted-foreground">{t("howItWorksSubtitle")}</p>
            </div>
            <div className="grid gap-8 sm:grid-cols-3">
              {steps.map((s, i) => (
                <Reveal key={s.n} delayMs={i * 100} className="space-y-3">
                  <p className="font-display text-4xl font-black text-accent">{s.n}</p>
                  <h3 className="font-display text-base font-bold">{s.title}</h3>
                  <p className="text-sm leading-relaxed text-muted-foreground">{s.text}</p>
                </Reveal>
              ))}
            </div>
          </section>
        </Reveal>

        <Reveal>
          <section className="space-y-8">
            <h2 className="font-display text-2xl font-extrabold tracking-tight sm:text-3xl">
              {t("faqTitle")}
            </h2>
            <div className="mx-auto max-w-2xl space-y-2">
              {faqs.map((f) => (
                <details
                  key={f.q}
                  className="group rounded-2xl border border-border bg-card p-5 transition-colors open:border-accent/40 [&_summary::-webkit-details-marker]:hidden"
                >
                  <summary className="flex cursor-pointer list-none items-center justify-between text-sm font-bold">
                    <span className="flex items-center gap-2.5">
                      <HelpCircle aria-hidden="true" className="h-4 w-4 shrink-0 text-accent" />
                      {f.q}
                    </span>
                    <ChevronRight
                      aria-hidden="true"
                      className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-90"
                    />
                  </summary>
                  <p className="mt-3 border-t border-border pt-3 text-sm leading-relaxed text-muted-foreground">
                    {f.a}
                  </p>
                </details>
              ))}
            </div>
          </section>
        </Reveal>

        <Reveal>
          <section className="border border-border bg-card px-6 py-10 text-center sm:px-10">
            <h2 className="font-display text-2xl font-extrabold sm:text-3xl">{t("bottomCtaTitle")}</h2>
            <p className="mx-auto mt-3 max-w-md text-sm text-muted-foreground">{t("bottomCtaText")}</p>
            <Link
              href={`/${locale}/login`}
              className={`${primaryCta} mt-6 inline-flex h-12 px-8 text-sm font-extrabold`}
            >
              {t("ctaStart")}
              <ChevronRight aria-hidden="true" className="ml-2 h-5 w-5" />
            </Link>
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
                    <Link href={item.href} className="text-foreground hover:text-accent">
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
                  href={`tel:${t("footerPhoneTel")}`}
                  className="inline-flex items-center gap-2 font-semibold text-foreground hover:text-accent"
                >
                  <Phone aria-hidden="true" className="h-4 w-4 text-accent" />
                  {t("footerPhone")}
                </a>
              </li>
              <li>
                <a
                  href={`mailto:${t("footerEmail")}`}
                  className="inline-flex items-center gap-2 font-semibold text-foreground hover:text-accent"
                >
                  <Mail aria-hidden="true" className="h-4 w-4 text-accent" />
                  {t("footerEmail")}
                </a>
              </li>
              <li className="flex items-start gap-2 text-muted-foreground">
                <MapPin aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0 text-accent" />
                <span>{t("footerAddress")}</span>
              </li>
              <li className="flex items-center gap-2 text-muted-foreground">
                <Clock3 aria-hidden="true" className="h-4 w-4 shrink-0 text-accent" />
                <span>{t("footerHours")}</span>
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
                  href={t("footerTelegramUrl")}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 text-foreground hover:text-accent"
                >
                  <Send aria-hidden="true" className="h-4 w-4 text-accent" />
                  {t("footerTelegram")}
                </a>
              </li>
              <li>
                <a
                  href={t("footerInstagramUrl")}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 text-foreground hover:text-accent"
                >
                  <Instagram aria-hidden="true" className="h-4 w-4 text-accent" />
                  {t("footerInstagram")}
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
