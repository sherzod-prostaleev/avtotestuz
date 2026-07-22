import { Button } from "@/components/ui/button";
import { proofStats } from "@/lib/mock-data";
import { DemoQuestionBlock } from "./demo-question-block";

const features = [
  { title: "FSRS o'quv dvigateli", text: "Har savol siz uchun aqlli takrorlash jadvali bilan due bo'ladi." },
  { title: "Tayyorlik %", text: "Imtihonga qanchalik tayyorligingizni real vaqtda ko'ring." },
  { title: "YHQ-havolali izohlar", text: "Har javobda huquqiy modda-havola bilan chuqur tushuntirish." },
  { title: "Real imtihon simulyatsiyasi", text: "20 savol, 25 daqiqa, 3-xatoda to'xtash — aynan rasmiy qoida." },
];

const steps = [
  { n: 1, title: "Ro'yxatdan o'ting", text: "Telefon raqamingiz bilan bir daqiqada." },
  { n: 2, title: "Bilet yeching", text: "Birinchi bilet har doim bepul." },
  { n: 3, title: "Tayyorlikni kuzating", text: "Statistikada zaif mavzularni ko'ring." },
];

const faqs = [
  { q: "Birinchi bilet chindan bepulmi?", a: "Ha, ro'yxatdan o'tgan har bir foydalanuvchi uchun 1-bilet doim bepul." },
  { q: "Savollar qayerdan olingan?", a: "Real imtihon savollari, litsenziyasi tasdiqlangan manbadan." },
  { q: "Necha tilda ishlaydi?", a: "uz-Latn, uz-Cyrl va rus tillarida." },
];

export default function LandingPage() {
  return (
    <main className="mx-auto max-w-5xl px-4 py-12">
      <section className="text-center">
        <h1 className="font-display text-4xl font-extrabold leading-tight md:text-6xl">
          Prava&apos;ni <span className="text-accent">oson</span> oling!
        </h1>
        <p className="mx-auto mt-4 max-w-xl text-lg text-muted-foreground">
          FSRS asosidagi aqlli o&apos;quv dvigateli bilan haydovchilik nazariy imtihoniga tayyorlaning.
        </p>
        <div className="mt-8 flex flex-col items-center gap-3 sm:flex-row sm:justify-center">
          <Button variant="game" size="lg">
            Bepul boshlash
          </Button>
          <span className="rounded-full border border-border px-4 py-1 text-sm text-muted-foreground">
            Ro&apos;yxatsiz sinab ko&apos;ring
          </span>
        </div>
      </section>

      <section className="mt-16">
        <h2 className="mb-6 text-center font-display text-2xl font-bold">Hozir sinab ko&apos;ring</h2>
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
        <h2 className="mb-6 text-center font-display text-2xl font-bold">Nega biz?</h2>
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
        <h2 className="mb-6 text-center font-display text-2xl font-bold">Qanday ishlaydi</h2>
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
        <h2 className="mb-6 text-center font-display text-2xl font-bold">Savol-javob</h2>
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
        AvtoTest — {new Date().getFullYear()}
      </footer>
    </main>
  );
}
