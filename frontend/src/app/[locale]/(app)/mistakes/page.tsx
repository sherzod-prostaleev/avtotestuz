"use client";

import { useState, useEffect } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { apiGet, ApiError } from "@/lib/api-client";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ArrowLeft, AlertTriangle, CheckCircle2, Lock, Play, RefreshCw } from "lucide-react";

interface MistakesData {
  due_count: number;
  total_bank_count: number;
}

export default function MistakesPage() {
  const t = useTranslations("Mistakes");
  const locale = useLocale();
  const router = useRouter();

  const [data, setData] = useState<MistakesData>({ due_count: 0, total_bank_count: 0 });
  const [loading, setLoading] = useState<boolean>(true);
  const [isVipLocked, setIsVipLocked] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function fetchMistakes() {
      setLoading(true);
      setError(null);
      try {
        const res = await apiGet<MistakesData>("me/mistakes");
        setData(res || { due_count: 0, total_bank_count: 0 });
      } catch (err: unknown) {
        if (err instanceof ApiError && err.status === 402) {
          setIsVipLocked(true);
        } else if (err instanceof ApiError) {
          setError(err.message);
        } else {
          // Default empty state for dev fallback
          setData({ due_count: 0, total_bank_count: 0 });
        }
      } finally {
        setLoading(false);
      }
    }
    fetchMistakes();
  }, []);

  const handleStart = () => {
    router.push(`/${locale}/session/start?mode=mistakes`);
  };

  return (
    <main className="mx-auto max-w-4xl px-4 py-8">
      <header className="mb-6">
        <Link href={`/${locale}/dashboard`} className="mb-2 flex items-center gap-1 text-sm text-accent hover:underline">
          <ArrowLeft className="h-4 w-4" /> Bosh sahifaga qaytish
        </Link>
        <h1 className="font-display text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </header>

      {isVipLocked ? (
        <Card className="p-8 text-center border-amber-500/30 bg-amber-500/5">
          <Lock className="mx-auto mb-3 h-12 w-12 text-gold" />
          <h2 className="font-display text-xl font-bold">{t("vipRequired")}</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            FSRS xatolar bankida takrorlash va xatolarni tuzatish imkoniyati Premium tarifi obunachilariga beriladi.
          </p>
          <div className="mt-6 flex justify-center gap-4">
            <Link href={`/${locale}/premium`}>
              <Button variant="game">Premiumga o'tish</Button>
            </Link>
          </div>
        </Card>
      ) : (
        <div className="space-y-6">
          {error && (
            <div className="rounded-md border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
              {error}
            </div>
          )}

          {/* 2 Numbers Stats */}
          <div className="grid gap-4 sm:grid-cols-2">
            <Card className="p-5 border-accent/40 bg-accent/5">
              <CardHeader className="p-0 pb-2">
                <CardTitle className="text-sm text-muted-foreground font-semibold">{t("dueNow")}</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <p className="font-display text-4xl font-extrabold text-accent">{data.due_count}</p>
              </CardContent>
            </Card>

            <Card className="p-5 border-border">
              <CardHeader className="p-0 pb-2">
                <CardTitle className="text-sm text-muted-foreground font-semibold">{t("totalBank")}</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <p className="font-display text-4xl font-extrabold">{data.total_bank_count}</p>
              </CardContent>
            </Card>
          </div>

          {/* FSRS Note / Empty State */}
          {data.due_count === 0 ? (
            <Card className="p-8 text-center border-border bg-card">
              <CheckCircle2 className="mx-auto mb-3 h-12 w-12 text-success" />
              <h2 className="font-display text-lg font-bold">{t("emptyDue")}</h2>
              <p className="mt-2 text-xs text-muted-foreground">{t("fsrsNote")}</p>
            </Card>
          ) : (
            <Card className="p-6">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="font-display text-lg font-bold">Xatolaringizni takrorlang</h3>
                  <p className="text-xs text-muted-foreground">{t("fsrsNote")}</p>
                </div>
                <Button variant="game" onClick={handleStart}>
                  <Play className="mr-2 h-4 w-4" /> {t("start")}
                </Button>
              </div>
            </Card>
          )}
        </div>
      )}
    </main>
  );
}
