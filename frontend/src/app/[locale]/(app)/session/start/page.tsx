"use client";

import { Suspense, useEffect } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useLocale } from "next-intl";
import { useSessionEngine } from "@/hooks/use-session-engine";

function SessionStartContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const locale = useLocale();
  const { startSession, error } = useSessionEngine();

  useEffect(() => {
    async function initSession() {
      const modeParam = searchParams.get("mode") as "variant" | "exam" | "practice" | "mistakes" | null;
      const variantParam = searchParams.get("variant_id");
      const categoryParam = searchParams.get("category_id");

      const mode = modeParam || (variantParam ? "variant" : "exam");
      const options: Record<string, unknown> = {};

      if (variantParam) options.variant_id = variantParam;
      if (categoryParam) options.category_id = categoryParam;

      const session = await startSession(mode, options);
      if (session?.id) {
        router.replace(`/${locale}/session/${session.id}`);
      }
    }

    initSession();
  }, [searchParams, router, locale, startSession]);

  if (error) {
    return (
      <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-6 text-center text-destructive">
        <p className="font-bold">Sessiyani boshlashda xatolik</p>
        <p className="mt-1 text-sm">{error}</p>
        <button
          onClick={() => router.push(`/${locale}/tickets`)}
          className="mt-4 rounded bg-accent px-4 py-2 text-xs font-bold text-accent-foreground"
        >
          Biletlarga qaytish
        </button>
      </div>
    );
  }

  return <div className="text-center text-muted-foreground animate-pulse">Test sessiyasi tayyorlanmoqda...</div>;
}

export default function SessionStartPage() {
  return (
    <main className="mx-auto flex min-h-[60vh] max-w-md items-center justify-center p-4">
      <Suspense fallback={<div className="text-center text-muted-foreground">Yuklanmoqda...</div>}>
        <SessionStartContent />
      </Suspense>
    </main>
  );
}
