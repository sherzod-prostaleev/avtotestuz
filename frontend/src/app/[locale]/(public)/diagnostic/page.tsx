"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft, ArrowRight, CheckCircle2, Clock3, RotateCw } from "lucide-react";
import { AnswerOption, type AnswerState } from "@/components/shared/answer-option";
import { BrandLogo } from "@/components/brand/brand-logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { recordDemoAnswer } from "@/lib/demo-progress-storage";

interface DiagnosticAnswer {
  id: string;
  text: string;
}

interface DiagnosticQuestion {
  id: string;
  text: string;
  image_url: string | null;
  answers: DiagnosticAnswer[];
}

interface DiagnosticPayload {
  questions: DiagnosticQuestion[];
  total_seconds: number;
}

interface GradePayload {
  correct: boolean;
  correct_answer_id: string;
}

interface Envelope<T> {
  data?: T;
  error?: { message?: string };
}

async function readData<T>(response: Response): Promise<T> {
  const payload = (await response.json()) as Envelope<T>;
  if (!response.ok || payload.data === undefined) {
    throw new Error(payload.error?.message || "diagnostic_request_failed");
  }
  return payload.data;
}

function formatTime(seconds: number): string {
  const safe = Math.max(0, seconds);
  const minutes = Math.floor(safe / 60);
  return `${String(minutes).padStart(2, "0")}:${String(safe % 60).padStart(2, "0")}`;
}

export default function PublicDiagnosticPage() {
  const locale = useLocale();
  const t = useTranslations("Diagnostic");
  const [questions, setQuestions] = useState<DiagnosticQuestion[]>([]);
  const [totalSeconds, setTotalSeconds] = useState(12 * 60);
  const [secondsLeft, setSecondsLeft] = useState(12 * 60);
  const [current, setCurrent] = useState(0);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [grade, setGrade] = useState<GradePayload | null>(null);
  const [correctCount, setCorrectCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [finished, setFinished] = useState(false);
  const [timedOut, setTimedOut] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const startedAt = useRef<number | null>(null);

  const loadDiagnostic = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch(
        `/api/proxy/demo/diagnostic?locale=${encodeURIComponent(locale)}`,
        { signal, cache: "no-store" },
      );
      const data = await readData<DiagnosticPayload>(response);
      if (data.questions.length === 0) throw new Error("empty_diagnostic");
      setQuestions(data.questions);
      setTotalSeconds(data.total_seconds);
      setSecondsLeft(data.total_seconds);
      setCurrent(0);
      setSelectedId(null);
      setGrade(null);
      setCorrectCount(0);
      setFinished(false);
      setTimedOut(false);
      startedAt.current = Date.now();
    } catch (loadError) {
      if (loadError instanceof DOMException && loadError.name === "AbortError") return;
      setError(t("loadError"));
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [locale, t]);

  useEffect(() => {
    const controller = new AbortController();
    void loadDiagnostic(controller.signal);
    return () => controller.abort();
  }, [loadDiagnostic]);

  useEffect(() => {
    if (loading || finished || startedAt.current === null) return;
    const tick = () => {
      const elapsed = Math.floor((Date.now() - (startedAt.current ?? Date.now())) / 1000);
      const remaining = Math.max(0, totalSeconds - elapsed);
      setSecondsLeft(remaining);
      if (remaining === 0) {
        setTimedOut(true);
        setFinished(true);
      }
    };
    tick();
    const id = window.setInterval(tick, 1000);
    return () => window.clearInterval(id);
  }, [finished, loading, totalSeconds]);

  const question = questions[current];
  const progress = questions.length > 0 ? ((current + (finished ? 1 : 0)) / questions.length) * 100 : 0;
  const percent = questions.length > 0 ? Math.round((correctCount / questions.length) * 100) : 0;
  const resultLabel = useMemo(() => {
    if (percent >= 80) return t("levelHigh");
    if (percent >= 50) return t("levelMedium");
    return t("levelStart");
  }, [percent, t]);

  async function submitAnswer(answerId: string) {
    if (!question || submitting || grade || finished) return;
    setSelectedId(answerId);
    setSubmitting(true);
    setError(null);
    try {
      const response = await fetch("/api/proxy/demo/answer", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ question_id: question.id, answer_id: answerId }),
      });
      const nextGrade = await readData<GradePayload>(response);
      setGrade(nextGrade);
      if (nextGrade.correct) setCorrectCount((count) => count + 1);
      recordDemoAnswer({
        questionId: question.id,
        answerId,
        correct: nextGrade.correct,
      });
    } catch {
      setSelectedId(null);
      setError(t("answerError"));
    } finally {
      setSubmitting(false);
    }
  }

  function nextQuestion() {
    if (!grade) return;
    if (current + 1 >= questions.length) {
      setFinished(true);
      return;
    }
    setCurrent((index) => index + 1);
    setSelectedId(null);
    setGrade(null);
    setError(null);
  }

  function stateFor(answerId: string): AnswerState {
    if (!selectedId) return "neutral";
    if (!grade) return answerId === selectedId ? "selected" : "neutral";
    if (answerId === grade.correct_answer_id) return "correct";
    if (answerId === selectedId) return "incorrect";
    return "neutral";
  }

  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="border-b border-border bg-card/80 backdrop-blur">
        <div className="mx-auto flex h-14 max-w-4xl items-center justify-between px-3 sm:px-4">
          <Link href={`/${locale}`} className="flex items-center gap-2 font-display font-black">
            <BrandLogo size={34} className="h-8 w-8 rounded-xl object-cover" />
            Driver Go
          </Link>
          <ThemeToggle />
        </div>
      </header>

      <main className="mx-auto max-w-4xl px-3 py-5 sm:px-4 sm:py-8">
        <div className="mb-5 flex items-center justify-between gap-3">
          <Link
            href={`/${locale}`}
            className="inline-flex min-h-11 items-center gap-1 text-sm font-bold text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft className="h-4 w-4" /> {t("back")}
          </Link>
          {!loading && !finished && (
            <span className="inline-flex items-center gap-2 rounded-xl border border-border bg-card px-3 py-2 font-mono text-sm font-bold tabular-nums">
              <Clock3 className="h-4 w-4 text-accent" /> {formatTime(secondsLeft)}
            </span>
          )}
        </div>

        {loading ? (
          <div className="rounded-2xl border border-border bg-card p-10 text-center text-sm text-muted-foreground">
            {t("loading")}
          </div>
        ) : error && questions.length === 0 ? (
          <div role="alert" className="rounded-2xl border border-destructive/30 bg-card p-8 text-center">
            <p className="text-sm text-destructive">{error}</p>
            <Button className="mt-4 gap-2" variant="outline" onClick={() => void loadDiagnostic()}>
              <RotateCw className="h-4 w-4" /> {t("retry")}
            </Button>
          </div>
        ) : finished ? (
          <section className="rounded-2xl border border-border bg-card p-6 text-center shadow-raised-sm sm:p-10">
            <CheckCircle2 className="mx-auto h-12 w-12 text-success" />
            <h1 className="mt-4 font-display text-3xl font-black">{t("resultTitle")}</h1>
            {timedOut && <p className="mt-2 text-sm font-semibold text-accent">{t("timedOut")}</p>}
            <p className="mt-5 text-5xl font-black tabular-nums">{correctCount}/{questions.length}</p>
            <p className="mt-2 text-lg font-bold text-accent">{resultLabel}</p>
            <p className="mx-auto mt-3 max-w-lg text-sm leading-relaxed text-muted-foreground">
              {t("resultBody", { percent })}
            </p>
            <div className="mt-7 flex flex-col justify-center gap-3 sm:flex-row">
              <Link
                href={`/${locale}/register`}
                className="inline-flex min-h-12 items-center justify-center gap-2 rounded-xl border-b-4 border-accent-shadow bg-accent px-6 text-sm font-extrabold text-accent-foreground shadow-3d"
              >
                {t("register")} <ArrowRight className="h-4 w-4" />
              </Link>
              <Button variant="outline" className="min-h-12 gap-2" onClick={() => void loadDiagnostic()}>
                <RotateCw className="h-4 w-4" /> {t("restart")}
              </Button>
            </div>
          </section>
        ) : question ? (
          <section className="rounded-2xl border border-border bg-card p-4 shadow-raised-sm sm:p-7">
            <div className="mb-5 h-2 overflow-hidden rounded-full bg-muted">
              <div className="h-full rounded-full bg-accent transition-[width]" style={{ width: `${progress}%` }} />
            </div>
            <div className="flex items-center justify-between gap-3 text-xs font-extrabold uppercase tracking-wider text-muted-foreground">
              <span>{t("question", { current: current + 1, total: questions.length })}</span>
              <span>{t("correct", { count: correctCount })}</span>
            </div>
            <h1 className="mt-4 font-display text-xl font-black leading-snug sm:text-2xl">{question.text}</h1>
            {question.image_url && (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={question.image_url} alt={t("imageAlt")} className="mx-auto mt-5 max-h-80 rounded-xl object-contain" />
            )}
            <div className="mt-6 space-y-3">
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
            {error && <p role="alert" className="mt-4 text-sm font-semibold text-destructive">{error}</p>}
            {grade && (
              <div className="mt-5 flex flex-col items-start justify-between gap-3 rounded-xl border border-border bg-muted/40 p-4 sm:flex-row sm:items-center">
                <p className={`text-sm font-bold ${grade.correct ? "text-success" : "text-destructive"}`}>
                  {grade.correct ? t("answerCorrect") : t("answerWrong")}
                </p>
                <Button className="gap-2" onClick={nextQuestion}>
                  {current + 1 === questions.length ? t("finish") : t("next")}
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </div>
            )}
          </section>
        ) : null}
      </main>
    </div>
  );
}
