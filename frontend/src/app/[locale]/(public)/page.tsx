import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { proofStats } from "@/lib/mock-data";
import { DemoQuestionBlock } from "./demo-question-block";

export default function LandingPage() {
  const t = useTranslations("Landing");

  const features = [
    { title: t("feature1Title"), text: t("feature1Text") },
    { title: t("feature2Title"), text: t("feature2Text") },
    { title: t("feature3Title"), text: t("feature3Text") },
    { title: t("feature4Title"), text: t("feature4Text") },
  ];
  const steps = [
    { n: 1, title: t("step1Title"), text: t("step1Text") },
    { n: 2, title: t("step2Title"), text: t("step2Text") },
    { n: 3, title: t("step3Title"), text: t("step3Text") },
  ];
  const faqs = [
    { q: t("faq1Q"), a: t("faq1A") },
    { q: t("faq2Q"), a: t("faq2A") },
    { q: t("faq3Q"), a: t("faq3A") },
  ];

  return (
    <main className="mx-auto max-w-5xl px-4 py-12">
      <section className="text-center">
        <h1 className="font-display text-4xl font-extrabold leading-tight md:text-6xl">
          {t("heroTitleBefore")} <span className="text-accent">{t("heroTitleAccent")}</span> {t("heroTitleAfter")}
        </h1>
        <p className="mx-auto mt-4 max-w-xl text-lg text-muted-foreground">{t("heroSubtitle")}</p>
        <div className="mt-8 flex flex-col items-center gap-3 sm:flex-row sm:justify-center">
          <Button variant="game" size="lg">
            {t("ctaStart")}
          </Button>
          <span className="rounded-full border border-border px-4 py-1 text-sm text-muted-foreground">
            {t("ctaNoSignup")}
          </span>
        </div>
      </section>

      <section className="mt-16">
        <h2 className="mb-6 text-center font-display text-2xl font-bold">{t("demoSectionTitle")}</h2>
        <DemoQuestionBlock />
      </section>

      <section className="mt-16 grid grid-cols-2 gap-6 text-center sm:grid-cols-4">
        {proofStats.map((s) => (
          <div key={s.label}>
            <p className="font-display text-3xl font-extrabold text-accent">{s.value}</p>
            <p className="text-sm text-muted-foreground">{s.label}</p>
          </div>
        ))}
      </section>

      <section className="mt-16">
        <h2 className="mb-6 text-center font-display text-2xl font-bold">{t("whyUsTitle")}</h2>
        <div className="grid gap-4 sm:grid-cols-2">
          {features.map((f) => (
            <div key={f.title} className="rounded-lg border border-border bg-card p-5">
              <h3 className="font-display font-bold">{f.title}</h3>
              <p className="mt-1 text-sm text-muted-foreground">{f.text}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="mt-16">
        <h2 className="mb-6 text-center font-display text-2xl font-bold">{t("howItWorksTitle")}</h2>
        <div className="grid gap-6 sm:grid-cols-3">
          {steps.map((s) => (
            <div key={s.n} className="text-center">
              <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-accent font-display font-bold text-accent-foreground">
                {s.n}
              </div>
              <h3 className="font-display font-bold">{s.title}</h3>
              <p className="mt-1 text-sm text-muted-foreground">{s.text}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="mt-16">
        <h2 className="mb-6 text-center font-display text-2xl font-bold">{t("faqTitle")}</h2>
        <div className="mx-auto max-w-2xl space-y-2">
          {faqs.map((f) => (
            <details key={f.q} className="rounded-lg border border-border bg-card p-4">
              <summary className="cursor-pointer font-semibold">{f.q}</summary>
              <p className="mt-2 text-sm text-muted-foreground">{f.a}</p>
            </details>
          ))}
        </div>
      </section>

      <footer className="mt-16 border-t border-border pt-6 text-center text-sm text-muted-foreground">
        {t("footer", { year: new Date().getFullYear() })}
      </footer>
    </main>
  );
}
