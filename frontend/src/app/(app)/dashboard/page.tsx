import { Button } from "@/components/ui/button";
import { ResultRing } from "@/components/shared/result-ring";
import { mockProfile } from "@/lib/mock-data";
import { Flame } from "lucide-react";

const navCards = [
  { title: "Biletlar", desc: "61 ta bilet, ketma-ket ochiladi" },
  { title: "Imtihon simulyatsiyasi", desc: "20 savol, 25 daqiqa" },
  { title: "Mashq", desc: "Mavzu yoki belgi bo'yicha" },
  { title: "Xatolar ustida ishlash", desc: "FSRS asosida takrorlash" },
];

export default function DashboardPage() {
  const { name, isVip, streak, readinessPercent } = mockProfile;
  return (
    <main className="mx-auto max-w-4xl px-4 py-8">
      <header className="flex items-center justify-between">
        <div>
          <p className="text-muted-foreground">Xush kelibsiz,</p>
          <h1 className="font-display text-2xl font-bold">{name}</h1>
        </div>
        <span
          className={
            isVip
              ? "rounded-full bg-gold px-4 py-1 text-sm font-bold text-background"
              : "rounded-full border border-border px-4 py-1 text-sm text-muted-foreground"
          }
        >
          {isVip ? "Premium" : "Bepul reja"}
        </span>
      </header>

      <section className="mt-6 grid gap-4 sm:grid-cols-2">
        <div className="rounded-lg border border-border bg-card p-5">
          <div className="flex items-center gap-2">
            <Flame className="h-6 w-6 text-streak" />
            <span className="font-display text-2xl font-extrabold">{streak.current}</span>
            <span className="text-muted-foreground">kunlik streak</span>
          </div>
          <div className="mt-3 h-2 w-full overflow-hidden rounded-full bg-border">
            <div
              className="h-full rounded-full bg-streak"
              style={{ width: `${Math.min(100, (streak.todayDone / streak.dailyGoal) * 100)}%` }}
            />
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            Bugun: {streak.todayDone}/{streak.dailyGoal} savol
          </p>
        </div>

        <div className="flex items-center justify-center rounded-lg border border-border bg-card p-5">
          <ResultRing percent={readinessPercent} label="Tayyorlik" />
        </div>
      </section>

      <section className="mt-6 grid gap-4 sm:grid-cols-2">
        {navCards.map((c) => (
          <Button
            key={c.title}
            variant="game"
            className="h-auto flex-col items-start gap-1 whitespace-normal px-5 py-4 text-left"
          >
            <span className="font-display text-base font-bold">{c.title}</span>
            <span className="text-xs font-normal opacity-80">{c.desc}</span>
          </Button>
        ))}
      </section>
    </main>
  );
}
