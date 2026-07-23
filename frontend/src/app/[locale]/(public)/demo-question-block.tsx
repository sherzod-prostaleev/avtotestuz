"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { AnswerOption, type AnswerState } from "@/components/shared/answer-option";
import { Button } from "@/components/ui/button";

interface DemoAnswer {
  id: string;
  position: number;
  text: string;
  image_url: string | null;
}

interface DemoQuestion {
  id: string;
  text: string;
  image_url: string | null;
  answers: DemoAnswer[];
}

interface DemoGrade {
  correct: boolean;
  correct_answer_id: string;
}

interface ApiEnvelope<T> {
  data?: T;
  error?: { code?: string; message?: string };
}

async function readData<T>(response: Response): Promise<T> {
  const payload = (await response.json()) as ApiEnvelope<T>;
  if (!response.ok || payload.data === undefined) {
    throw new Error(payload.error?.message || "demo_request_failed");
  }
  return payload.data;
}

export function DemoQuestionBlock() {
  const locale = useLocale();
  const t = useTranslations("Landing");
  const [question, setQuestion] = useState<DemoQuestion | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [grade, setGrade] = useState<DemoGrade | null>(null);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadQuestion = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError(null);
    setQuestion(null);
    setSelectedId(null);
    setGrade(null);

    try {
      const response = await fetch(`/api/proxy/demo/question?locale=${encodeURIComponent(locale)}`, {
        signal,
      });
      const nextQuestion = await readData<DemoQuestion>(response);
      setQuestion(nextQuestion);
    } catch (loadError) {
      if (loadError instanceof DOMException && loadError.name === "AbortError") return;
      setError(t("demoLoadError"));
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [locale, t]);

  useEffect(() => {
    const controller = new AbortController();
    void loadQuestion(controller.signal);
    return () => controller.abort();
  }, [loadQuestion]);

  async function submitAnswer(answerId: string) {
    if (!question || submitting || grade) return;

    setSelectedId(answerId);
    setSubmitting(true);
    setError(null);
    try {
      const response = await fetch("/api/proxy/demo/answer", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ question_id: question.id, answer_id: answerId }),
      });
      setGrade(await readData<DemoGrade>(response));
    } catch {
      setError(t("demoAnswerError"));
    } finally {
      setSubmitting(false);
    }
  }

  function stateFor(answerId: string): AnswerState {
    if (!selectedId) return "neutral";
    if (!grade) return answerId === selectedId ? "selected" : "neutral";
    if (answerId === grade.correct_answer_id) return "correct";
    if (answerId === selectedId) return "incorrect";
    return "neutral";
  }

  if (loading) {
    return <div className="py-12 text-center text-sm text-muted-foreground">{t("demoLoading")}</div>;
  }

  if (!question) {
    return (
      <div
        role="alert"
        className="rounded-2xl border border-destructive/30 bg-destructive/5 p-6 text-center"
      >
        <p className="text-sm text-destructive">{error || t("demoLoadError")}</p>
        <Button className="mt-4" variant="outline" size="sm" onClick={() => void loadQuestion()}>
          {t("demoRetry")}
        </Button>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-xl" aria-busy={submitting}>
      <div className="space-y-4">
        <span className="inline-flex rounded-full border border-accent/30 bg-accent/10 px-3.5 py-1 text-xs font-extrabold text-accent">
          {t("demoQuestionNumber", { current: 1, total: 1 })}
        </span>
        <h3 className="font-display text-lg font-bold leading-snug tracking-tight sm:text-xl">
          {question.text}
        </h3>
        {question.image_url && (
          <div className="overflow-hidden rounded-2xl border border-border bg-black/5">
            {/* Demo media URLs are backend-owned and intentionally unoptimized. */}
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={question.image_url}
              alt={t("demoImageAlt")}
              className="mx-auto max-h-72 w-full object-contain"
            />
          </div>
        )}
      </div>
      <div className="mt-4 flex flex-col gap-3">
        {question.answers.map((answer, index) => (
          <AnswerOption
            key={answer.id}
            id={answer.id}
            shortcutLabel={`F${index + 1}`}
            text={answer.text}
            state={stateFor(answer.id)}
            disabled={submitting || Boolean(grade)}
            onSelect={(answerId) => void submitAnswer(answerId)}
          />
        ))}
      </div>
      {error && question && (
        <p role="alert" className="mt-3 text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  );
}
