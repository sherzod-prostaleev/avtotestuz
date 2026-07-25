"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import Link from "next/link";
import { RefreshCw, Search } from "lucide-react";
import { Button } from "@/components/ui/button";

type QuestionRow = {
  id: string;
  source_ext_id: string;
  category_code: string;
  category_name: string;
  validation_status: string;
  text_preview: string;
  variant_numbers: number[];
  explanation_status: string;
  updated_at: string;
};

type ListPayload = {
  items: QuestionRow[];
  page: number;
  limit: number;
  total: number;
};

export default function AdminQuestionsPage() {
  const t = useTranslations("AdminContent");
  const locale = useLocale();
  const [q, setQ] = useState("");
  const [validation, setValidation] = useState("");
  const [explanation, setExplanation] = useState("");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<ListPayload | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(
    async (query: string, pageNum: number, val: string, expl: string) => {
      setLoading(true);
      setError(null);
      try {
        const params = new URLSearchParams({
          page: String(pageNum),
          limit: "50",
        });
        if (query.trim()) params.set("q", query.trim());
        if (val) params.set("validation", val);
        if (expl) params.set("explanation", expl);
        const res = await fetch(`/api/admin/content/questions?${params}`, { cache: "no-store" });
        const json = await res.json();
        if (!res.ok) {
          setError(json?.error?.code === "forbidden" ? t("errorForbidden") : t("errorLoad"));
          setData(null);
          return;
        }
        setData(json.data as ListPayload);
      } catch {
        setError(t("errorLoad"));
        setData(null);
      } finally {
        setLoading(false);
      }
    },
    [t],
  );

  useEffect(() => {
    void load(q, page, validation, explanation);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- search on submit / page/filter change
  }, [page, validation, explanation, load]);

  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.limit)) : 1;

  return (
    <main className="mx-auto max-w-5xl space-y-4">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("questionsTitle")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("questionsSubtitle")}</p>
        </div>
        <Link
          href={`/${locale}/admin/content/explanations`}
          className="text-sm font-semibold text-accent hover:underline"
        >
          {t("explanationsLink")}
        </Link>
      </header>

      <form
        className="flex flex-wrap gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          setPage(1);
          void load(q, 1, validation, explanation);
        }}
      >
        <div className="relative min-w-0 flex-1 basis-48">
          <Search
            aria-hidden
            className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
          />
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder={t("searchPlaceholder")}
            className="h-10 w-full rounded-xl border border-border bg-card pl-9 pr-3 text-sm"
          />
        </div>
        <select
          value={validation}
          onChange={(e) => {
            setPage(1);
            setValidation(e.target.value);
          }}
          className="h-10 rounded-xl border border-border bg-card px-3 text-sm"
          aria-label={t("filterValidation")}
        >
          <option value="">{t("filterAll")}</option>
          <option value="valid">valid</option>
          <option value="quarantined">quarantined</option>
        </select>
        <select
          value={explanation}
          onChange={(e) => {
            setPage(1);
            setExplanation(e.target.value);
          }}
          className="h-10 rounded-xl border border-border bg-card px-3 text-sm"
          aria-label={t("filterExplanation")}
        >
          <option value="">{t("filterAll")}</option>
          <option value="none">none</option>
          <option value="draft">draft</option>
          <option value="pending">pending</option>
          <option value="verified">verified</option>
        </select>
        <Button type="submit" size="sm" disabled={loading}>
          {t("search")}
        </Button>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={loading}
          onClick={() => void load(q, page, validation, explanation)}
        >
          <RefreshCw aria-hidden className={`mr-1.5 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          {t("refresh")}
        </Button>
      </form>

      {error ? (
        <p className="text-sm text-destructive">{error}</p>
      ) : !data ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : data.items.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("empty")}</p>
      ) : (
        <>
          <div className="overflow-x-auto rounded-xl border border-border">
            <table className="w-full min-w-[720px] text-left text-sm">
              <thead className="border-b border-border bg-card text-xs uppercase tracking-wide text-muted-foreground">
                <tr>
                  <th className="px-3 py-2 font-bold">{t("colExtId")}</th>
                  <th className="px-3 py-2 font-bold">{t("colCategory")}</th>
                  <th className="px-3 py-2 font-bold">{t("colText")}</th>
                  <th className="px-3 py-2 font-bold">{t("colVariants")}</th>
                  <th className="px-3 py-2 font-bold">{t("colValidation")}</th>
                  <th className="px-3 py-2 font-bold">{t("colExplanation")}</th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((row) => (
                  <tr key={row.id} className="border-b border-border/60 last:border-0 hover:bg-card/60">
                    <td className="px-3 py-2 font-mono text-xs">
                      <Link
                        href={`/${locale}/admin/content/questions/${row.id}`}
                        className="font-semibold text-accent hover:underline"
                      >
                        {row.source_ext_id}
                      </Link>
                    </td>
                    <td className="px-3 py-2 text-xs">{row.category_code}</td>
                    <td className="max-w-xs truncate px-3 py-2">{row.text_preview || "—"}</td>
                    <td className="px-3 py-2 tabular-nums text-xs">
                      {row.variant_numbers?.length ? row.variant_numbers.join(", ") : "—"}
                    </td>
                    <td className="px-3 py-2">
                      <span
                        className={`text-xs font-bold uppercase ${
                          row.validation_status === "quarantined"
                            ? "text-destructive"
                            : "text-muted-foreground"
                        }`}
                      >
                        {row.validation_status}
                      </span>
                    </td>
                    <td className="px-3 py-2 text-xs font-semibold uppercase text-muted-foreground">
                      {row.explanation_status}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <p>
              {t("total", { count: data.total })} · {t("pageOf", { page: data.page, pages: totalPages })}
            </p>
            <div className="flex gap-2">
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={page <= 1 || loading}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                {t("prev")}
              </Button>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={page >= totalPages || loading}
                onClick={() => setPage((p) => p + 1)}
              >
                {t("next")}
              </Button>
            </div>
          </div>
        </>
      )}
    </main>
  );
}
