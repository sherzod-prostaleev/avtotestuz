"use client";

import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { ImageOff } from "lucide-react";
import type { RoadSign } from "@/hooks/use-signs";
import { Card } from "@/components/ui/card";

function practiceHref(locale: string, code: string, count: number): string {
  const params = new URLSearchParams({
    mode: "practice",
    sign_id: code,
    count: String(Math.max(count, 1)),
  });
  return `/${locale}/session/start?${params.toString()}`;
}

export function SignPracticeGrid({
  signs,
  emptyHint,
}: {
  signs: RoadSign[];
  emptyHint?: string;
}) {
  const t = useTranslations("Signs");
  const locale = useLocale();
  const linked = signs.filter((sign) => sign.question_count > 0);

  if (linked.length === 0) {
    return (
      <Card className="p-5 text-center text-sm text-muted-foreground" role="status">
        {emptyHint ?? t("noQuestionsForSign")}
      </Card>
    );
  }

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
      {linked.map((sign) => (
        <Link
          key={sign.code}
          href={practiceHref(locale, sign.code, sign.question_count)}
          className="surface-raised-sm surface-interactive group relative overflow-hidden rounded-2xl border border-border bg-card p-3 text-center hover:border-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={`${sign.code}. ${sign.name}. ${t("questionCountLabel", { count: sign.question_count })}`}
        >
          <span className="absolute right-2 top-2 inline-flex min-h-7 min-w-7 items-center justify-center rounded-full bg-accent px-1.5 text-xs font-extrabold text-accent-foreground shadow-sm">
            {t("questionCountBadge", { count: sign.question_count })}
          </span>
          <div className="mx-auto mb-2 flex h-16 w-16 items-center justify-center rounded-xl bg-black/5 p-1.5">
            {sign.image_url ? (
              // Dynamic media URLs are served by the backend and intentionally stay unoptimized.
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={sign.image_url}
                alt=""
                className="max-h-full max-w-full object-contain"
              />
            ) : (
              <ImageOff className="h-6 w-6 text-muted-foreground" aria-hidden="true" />
            )}
          </div>
          <span className="block text-xs font-extrabold text-accent">{sign.code}</span>
          <span className="mt-1 line-clamp-2 text-sm font-bold text-foreground group-hover:text-accent">
            {sign.name}
          </span>
        </Link>
      ))}
    </div>
  );
}
