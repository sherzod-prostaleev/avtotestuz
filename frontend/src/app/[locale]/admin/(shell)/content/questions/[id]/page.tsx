"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";

type TranslationSummary = {
  locale: string;
  text?: string;
  status: string;
};

type AnswerSummary = {
  id: string;
  position: number;
  is_correct: boolean;
  translations: TranslationSummary[];
};

type QuestionDetail = {
  id: string;
  source_ext_id: string;
  category_code: string;
  category_name: string;
  validation_status: string;
  source: string;
  content_hash: string;
  created_at: string;
  updated_at: string;
  translations: TranslationSummary[];
  answers: AnswerSummary[];
  variants: { number: number; position: number }[];
  explanation?: {
    id: string;
    locales: TranslationSummary[];
    blocks_preview?: string;
    verified_at?: string;
  };
};

export default function AdminQuestionDetailPage({ params }: { params: { id: string } }) {
  const t = useTranslations("AdminContent");
  const locale = useLocale();
  const { id } = params;
  const [q, setQ] = useState<QuestionDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setError(null);
    try {
      const res = await fetch(`/api/admin/content/questions/${id}`, { cache: "no-store" });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.code === "not_found" ? t("errorNotFound") : t("errorLoad"));
        setQ(null);
        return;
      }
      setQ(json.data as QuestionDetail);
    } catch {
      setError(t("errorLoad"));
    }
  }, [id, t]);

  useEffect(() => {
    void load();
  }, [load]);

  async function patchValidation(status: string) {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/content/questions/${id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ validation_status: status }),
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.message ?? t("errorAction"));
        return;
      }
      setQ(json.data as QuestionDetail);
    } catch {
      setError(t("errorAction"));
    } finally {
      setBusy(false);
    }
  }

  async function verifyExplanation() {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/content/questions/${id}/explanation/verify`, {
        method: "POST",
      });
      const json = await res.json();
      if (!res.ok) {
        setError(json?.error?.message ?? t("errorAction"));
        return;
      }
      setQ(json.data as QuestionDetail);
    } catch {
      setError(t("errorAction"));
    } finally {
      setBusy(false);
    }
  }

  const explStatus = q?.explanation?.locales?.find((l) => l.locale === "uz-Latn")?.status;

  return (
    <main className="mx-auto max-w-3xl space-y-4">
      <Link
        href={`/${locale}/admin/content/questions`}
        className="inline-flex items-center gap-1 text-sm font-semibold text-muted-foreground hover:text-accent"
      >
        <ArrowLeft aria-hidden className="h-3.5 w-3.5" />
        {t("backQuestions")}
      </Link>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}
      {!q && !error ? <p className="text-sm text-muted-foreground">{t("loading")}</p> : null}

      {q ? (
        <>
          <header>
            <p className="font-mono text-xs text-muted-foreground">{q.source_ext_id}</p>
            <h1 className="font-display text-2xl font-extrabold tracking-tight">
              {q.translations.find((tr) => tr.locale === "uz-Latn")?.text || q.source_ext_id}
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">
              {q.category_name} ({q.category_code}) · {q.validation_status}
            </p>
          </header>

          <section className="space-y-2 rounded-xl border border-border bg-card p-4">
            <h2 className="text-sm font-bold uppercase tracking-wide text-muted-foreground">
              {t("actionsTitle")}
            </h2>
            <div className="flex flex-wrap gap-2">
              {q.validation_status === "valid" ? (
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={busy}
                  onClick={() => void patchValidation("quarantined")}
                >
                  {t("quarantine")}
                </Button>
              ) : (
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={busy}
                  onClick={() => void patchValidation("valid")}
                >
                  {t("restoreValid")}
                </Button>
              )}
              {q.explanation && explStatus !== "verified" ? (
                <Button type="button" size="sm" disabled={busy} onClick={() => void verifyExplanation()}>
                  {t("verifyExplanation")}
                </Button>
              ) : null}
            </div>
          </section>

          <section className="space-y-2">
            <h2 className="text-sm font-bold uppercase tracking-wide text-muted-foreground">
              {t("translationsTitle")}
            </h2>
            <ul className="space-y-2">
              {q.translations.map((tr) => (
                <li key={tr.locale} className="rounded-xl border border-border px-3 py-2 text-sm">
                  <span className="font-mono text-xs text-muted-foreground">
                    {tr.locale} · {tr.status}
                  </span>
                  <p className="mt-1">{tr.text}</p>
                </li>
              ))}
            </ul>
          </section>

          <section className="space-y-2">
            <h2 className="text-sm font-bold uppercase tracking-wide text-muted-foreground">
              {t("answersTitle")}
            </h2>
            <ol className="space-y-2">
              {q.answers.map((a) => (
                <li key={a.id} className="rounded-xl border border-border px-3 py-2 text-sm">
                  <span className="text-xs font-bold uppercase text-muted-foreground">
                    #{a.position}
                    {a.is_correct ? ` · ${t("correct")}` : ""}
                  </span>
                  <p className="mt-1">
                    {a.translations.find((tr) => tr.locale === "uz-Latn")?.text ||
                      a.translations[0]?.text ||
                      "—"}
                  </p>
                </li>
              ))}
            </ol>
          </section>

          <section className="space-y-2">
            <h2 className="text-sm font-bold uppercase tracking-wide text-muted-foreground">
              {t("variantsTitle")}
            </h2>
            {q.variants.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t("noVariants")}</p>
            ) : (
              <p className="text-sm">
                {q.variants.map((v) => `#${v.number}@${v.position}`).join(", ")}
              </p>
            )}
          </section>

          <section className="space-y-2">
            <h2 className="text-sm font-bold uppercase tracking-wide text-muted-foreground">
              {t("explanationTitle")}
            </h2>
            {!q.explanation ? (
              <p className="text-sm text-muted-foreground">{t("noExplanation")}</p>
            ) : (
              <div className="rounded-xl border border-border px-3 py-2 text-sm">
                <p className="text-xs font-semibold uppercase text-muted-foreground">
                  {q.explanation.locales.map((l) => `${l.locale}:${l.status}`).join(" · ")}
                </p>
                {q.explanation.blocks_preview ? (
                  <p className="mt-2 text-muted-foreground">{q.explanation.blocks_preview}</p>
                ) : null}
              </div>
            )}
          </section>
        </>
      ) : null}
    </main>
  );
}
