"use client";

import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { proofStats } from "@/lib/mock-data";
import { DemoQuestionBlock } from "./demo-question-block";
import { ThemeToggle } from "@/components/theme-toggle";
import {
  Sparkles,
  Award,
  BookOpen,
  BrainCircuit,
  ShieldCheck,
  Zap,
  HelpCircle,
  ChevronRight,
  LogIn,
  CheckCircle2,
  Smartphone,
} from "lucide-react";

export default function LandingPage() {
  const t = useTranslations("Landing");
  const locale = useLocale();

  const features = [
    {
      icon: BrainCircuit,
      color: "text-violet-500",
      bg: "bg-violet-500/10",
      title: "FSRS 4.5 Aqlli O'quv Dvigateli",
      text: "Kognitiv psixologiya va spaced repetition algoritmi asosida xatolaringizni eng o'z vaqtida takrorlashga beradi.",
    },
    {
      icon: Award,
      color: "text-amber-500",
      bg: "bg-amber-500/10",
      title: "Real Imtihon Simulyatsiyasi",
      text: "YHXB rasmiy imtihon qoidalari: 20 savol, 25 daqiqa, 3-xatoda avtostop. Hayajonsiz tayyorgarlik.",
    },
    {
      icon: ShieldCheck,
      color: "text-emerald-500",
      bg: "bg-emerald-500/10",
      title: "100% Rasmiy YHQ 2026 Bazasi",
      text: "O'zbekiston Respublikasi YHQ barcha 1235 ta rasmiy nazariy savoli va 61 ta bileti.",
    },
    {
      icon: Zap,
      color: "text-sky-500",
      bg: "bg-sky-500/10",
      title: "Huquqiy Modda Tushuntirishlari",
      text: "Har bir javobda tegishli YHQ moddasi keltiriladi. Tushunib o'rganing, yodlab emas.",
    },
  ];

  const steps = [
    {
      n: "1",
      icon: Smartphone,
      title: "Telefon bilan kiring",
      text: "Parolsiz, SMS orqali 10 soniyada ro'yxatdan o'ting.",
    },
    {
      n: "2",
      icon: BookOpen,
      title: "Bilet va mashq yeching",
      text: "61 ta bilet, FSRS xatolar banki va kategoriya mashqlari bilan tayyorlaning.",
    },
    {
      n: "3",
      icon: CheckCircle2,
      title: "Imtihondan o'ting!",
      text: "Simulyatorda 100% tayyorligingizni tekshirib, birinchi urinishda prava oling.",
    },
  ];

  const faqs = [
    { q: t("faq1Q"), a: t("faq1A") },
    { q: t("faq2Q"), a: t("faq2A") },
    { q: t("faq3Q"), a: t("faq3A") },
    {
      q: "Qaysi toifa (A, B, C, D) uchun savollar bor?",
      a: "Barcha asosiy toifalar uchun rasmiy YHQ nazariy imtihon savollari to'liq qamrab olingan.",
    },
  ];

  return (
    <div className="min-h-screen bg-background text-foreground flex flex-col">
      {/* ───── Top Navigation Bar ───── */}
      <header className="sticky top-0 z-40 w-full border-b border-border/60 bg-background/80 backdrop-blur-xl">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-4">
          <Link href={`/${locale}`} className="flex items-center gap-2.5 font-display text-xl font-black tracking-tight">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-accent text-white shadow-3d transition-transform hover:scale-105">
              🚗
            </div>
            <span className="bg-gradient-to-r from-accent via-indigo-500 to-accent bg-clip-text text-transparent">
              AvtoTest
            </span>
          </Link>

          <div className="flex items-center gap-3">
            <ThemeToggle />
            <Link href={`/${locale}/login`}>
              <Button variant="game" size="sm" className="px-5">
                <LogIn className="mr-1.5 h-3.5 w-3.5" /> Kirish
              </Button>
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
            <Sparkles className="h-4 w-4" /> 2026-yil Yangilangan Rasmiy Savollar Bazasi
          </div>

          {/* Main Title */}
          <h1 className="mx-auto max-w-4xl font-display text-4xl sm:text-5xl md:text-6xl font-black leading-[1.1] tracking-tight animate-fade-in">
            Haydovchilik imtihoniga{" "}
            <span className="bg-gradient-to-r from-accent via-indigo-500 to-accent bg-clip-text text-transparent">
              100% aqlli va oson
            </span>{" "}
            tayyorlaning!
          </h1>

          {/* Subtitle */}
          <p className="mx-auto max-w-2xl text-base sm:text-lg text-muted-foreground leading-relaxed animate-fade-in">
            O'zbekiston YHQ 1235 ta rasmiy savoli, 61 ta bilet, FSRS aqlli takrorlash va real imtihon simulyatori — hammasini bitta platformada.
          </p>

          {/* CTA Buttons */}
          <div className="flex flex-col items-center justify-center gap-4 sm:flex-row animate-fade-in">
            <Link href={`/${locale}/login`}>
              <Button variant="game" size="lg" className="px-10 py-4 text-base font-extrabold shadow-xl">
                Bepul Boshlash <ChevronRight className="ml-2 h-5 w-5" />
              </Button>
            </Link>
            <Link href={`/${locale}/signs`}>
              <Button variant="outline" size="lg" className="px-8 py-4 text-sm font-bold">
                Yo'l belgilari katalogi
              </Button>
            </Link>
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
        {/* ───── DEMO SECTION ───── */}
        <section className="space-y-8">
          <div className="text-center space-y-3">
            <span className="inline-block rounded-full bg-accent/10 px-4 py-1.5 text-xs font-extrabold text-accent">
              Interaktiv Sinov
            </span>
            <h2 className="font-display text-2xl sm:text-3xl font-extrabold tracking-tight">{t("demoSectionTitle")}</h2>
            <p className="mx-auto max-w-lg text-sm text-muted-foreground">
              Ro'yxatdan o'tmasdan turib haqiqiy imtihon savolini yechib ko'ring.
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
              Minglab bo'lajak haydovchilar AvtoTest platformasini tanlaydi.
            </p>
          </div>
          <div className="grid gap-6 sm:grid-cols-2">
            {features.map((f) => {
              const Icon = f.icon;
              return (
                <div key={f.title} className="glass-card p-6 flex items-start gap-5 hover:border-accent/50 hover:shadow-xl hover:-translate-y-0.5">
                  <div className={`flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl ${f.bg} ${f.color} shadow-sm`}>
                    <Icon className="h-6 w-6" />
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
              Imtihonga tayyorlanish 3 ta oddiy bosqichdan iborat.
            </p>
          </div>
          <div className="grid gap-6 sm:grid-cols-3">
            {steps.map((s) => {
              const Icon = s.icon;
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
                    <HelpCircle className="h-4 w-4 text-accent shrink-0" /> {f.q}
                  </span>
                  <ChevronRight className="h-4 w-4 text-muted-foreground transition-transform group-open:rotate-90 shrink-0" />
                </summary>
                <p className="mt-3 pt-3 border-t border-border text-xs leading-relaxed text-muted-foreground">{f.a}</p>
              </details>
            ))}
          </div>
        </section>

        {/* ───── BOTTOM CTA ───── */}
        <section className="rounded-3xl border border-accent/30 bg-gradient-to-br from-card via-card to-accent/10 p-10 text-center space-y-5 shadow-xl">
          <h2 className="font-display text-2xl sm:text-3xl font-extrabold">Hoziroq o'qishni boshlang!</h2>
          <p className="mx-auto max-w-md text-sm text-muted-foreground">
            Bepul ro'yxatdan o'ting va birinchi urinishda imtihondan o'ting.
          </p>
          <Link href={`/${locale}/login`}>
            <Button variant="game" size="lg" className="px-10 py-4 text-base font-extrabold shadow-xl">
              Bepul Boshlash <ChevronRight className="ml-2 h-5 w-5" />
            </Button>
          </Link>
        </section>
      </main>

      {/* ───── FOOTER ───── */}
      <footer className="border-t border-border bg-card/50 py-8">
        <div className="mx-auto max-w-6xl px-4 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-muted-foreground">
          <div className="flex items-center gap-2 font-display font-extrabold text-foreground">
            🚗 AvtoTest · {new Date().getFullYear()}
          </div>
          <p>O'zbekiston Respublikasi YHQ rasmiy o'quv platformasi</p>
        </div>
      </footer>
    </div>
  );
}
