"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { useParams, useRouter } from "next/navigation";
import {
  AlertTriangle,
  Award,
  Bookmark,
  Check,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Clock3,
  Expand,
  LoaderCircle,
  Minimize2,
  RefreshCw,
  X,
  XCircle,
} from "lucide-react";
import { locales } from "@/i18n/config";
import { apiDelete, apiGet, apiPost } from "@/lib/api-client";
import { trackEvent, type SafeAnalyticsProps } from "@/lib/analytics-events";
import {
  useSessionEngine,
  type SessionMode,
  type SessionQuestionItem,
} from "@/hooks/use-session-engine";
import { AnswerOption, type AnswerState } from "@/components/shared/answer-option";
import { CountdownTimer } from "@/components/shared/countdown-timer";
import { QuestionCard } from "@/components/shared/question-card";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

interface SavedItemDTO {
  question_id: string;
}

interface PendingAnswer {
  questionId: string;
  answerId: string;
}

function hasAnswer(question: SessionQuestionItem): boolean {
  return question.answered === true || Boolean(question.user_answer_id);
}

function answerState(question: SessionQuestionItem, answerId: string): AnswerState {
  if (question.correct_answer_id === answerId) return "correct";
  if (question.user_answer_id === answerId) {
    return question.correct === false ? "wrong" : "selected";
  }
  return "neutral";
}

export default function TestSessionPage() {
  const params = useParams();
  const router = useRouter();
  const locale = useLocale();
  const t = useTranslations("Session");
  const sessionId = typeof params.id === "string" ? params.id : "";
  const { session, loading, submitting, error, loadSession, submitAnswer, finishSession } =
    useSessionEngine(sessionId);

  const [currentIndex, setCurrentIndex] = useState(0);
  const [zoomImageUrl, setZoomImageUrl] = useState<string | null>(null);
  const [pendingAnswer, setPendingAnswer] = useState<PendingAnswer | null>(null);
  const [savedIds, setSavedIds] = useState<Set<string>>(new Set());
  const [bookmarkBusy, setBookmarkBusy] = useState(false);
  const [bookmarkError, setBookmarkError] = useState(false);
  const [finishing, setFinishing] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const initializedSessionRef = useRef<string | null>(null);
  const viewedQuestionsRef = useRef<Set<string>>(new Set());

  useEffect(() => {
    if (sessionId) void loadSession(sessionId, locale);
  }, [sessionId, locale, loadSession]);

  useEffect(() => {
    let active = true;
    apiGet<SavedItemDTO[]>("me/saved")
      .then((items) => {
        if (active) setSavedIds(new Set(items.map((item) => item.question_id)));
      })
      .catch(() => {
        if (active) setSavedIds(new Set());
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!session || initializedSessionRef.current === session.id) return;
    initializedSessionRef.current = session.id;
    const firstUnanswered = session.questions.findIndex((question) => !hasAnswer(question));
    setCurrentIndex(firstUnanswered >= 0 ? firstUnanswered : 0);
  }, [session]);

  useEffect(() => {
    const onFullscreenChange = () => setIsFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener("fullscreenchange", onFullscreenChange);
    return () => document.removeEventListener("fullscreenchange", onFullscreenChange);
  }, []);

  const handleSelectAnswer = useCallback(
    async (questionId: string, answerId: string) => {
      const question = session?.questions.find((item) => item.id === questionId);
      if (!session || session.status === "completed" || !question || hasAnswer(question) || submitting) {
        return;
      }

      const response = await submitAnswer(sessionId, questionId, answerId);
      if (!response) {
        setPendingAnswer({ questionId, answerId });
        return;
      }

      setPendingAnswer(null);
      const answerProps: SafeAnalyticsProps = {
        session_id: session.id,
        question_id: questionId,
        mode: session.mode,
        position: question.position ?? session.questions.indexOf(question) + 1,
        status: response.recorded ? "recorded" : "not_recorded",
      };
      if (response.correct !== undefined) answerProps.correct = response.correct;
      trackEvent("answer", answerProps);
      if (response.stopped) {
        trackEvent("session_finish", {
          session_id: session.id,
          mode: session.mode,
          status: "failed",
          stopped_reason: response.stop_reason ?? "too_many_errors",
          total: session.total,
        });
      }
    },
    [session, sessionId, submitAnswer, submitting]
  );

  const handleFinish = useCallback(async () => {
    if (!session || session.status === "completed" || finishing || submitting) return;
    setFinishing(true);
    setPendingAnswer(null);
    try {
      const completed = await finishSession(sessionId);
      if (completed) {
        trackEvent("session_finish", {
          session_id: completed.id,
          mode: completed.mode,
          status: completed.passed ? "passed" : "failed",
          score: completed.score,
          total: completed.total,
          stopped_reason: completed.stopped_reason,
        });
      }
    } finally {
      setFinishing(false);
    }
  }, [finishSession, finishing, session, sessionId, submitting]);

  const questions = session?.questions ?? [];
  const currentQuestion = questions[currentIndex];
  const currentAnswered = currentQuestion ? hasAnswer(currentQuestion) : false;

  useEffect(() => {
    if (!session || !currentQuestion || session.status === "completed") return;
    const eventKey = `${session.id}:${currentQuestion.id}`;
    if (viewedQuestionsRef.current.has(eventKey)) return;
    viewedQuestionsRef.current.add(eventKey);
    trackEvent("view_question", {
      session_id: session.id,
      question_id: currentQuestion.id,
      mode: session.mode,
      position: currentIndex + 1,
      locale,
    });
  }, [currentIndex, currentQuestion, locale, session]);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && zoomImageUrl) {
        setZoomImageUrl(null);
        return;
      }
      if (!session || session.status === "completed" || !currentQuestion) return;

      if (event.key === "ArrowLeft" && currentIndex > 0) {
        event.preventDefault();
        setCurrentIndex((value) => Math.max(0, value - 1));
        return;
      }
      if (event.key === "ArrowRight" && currentAnswered && currentIndex < questions.length - 1) {
        event.preventDefault();
        setCurrentIndex((value) => Math.min(questions.length - 1, value + 1));
        return;
      }

      const shortcutIndex = ["F1", "F2", "F3", "F4", "F5"].indexOf(event.key);
      if (shortcutIndex >= 0 && !currentAnswered && currentQuestion.answers[shortcutIndex]) {
        event.preventDefault();
        const answer = currentQuestion.answers[shortcutIndex];
        void handleSelectAnswer(currentQuestion.id, answer.id);
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [
    currentAnswered,
    currentIndex,
    currentQuestion,
    handleSelectAnswer,
    questions.length,
    session,
    zoomImageUrl,
  ]);

  const toggleBookmark = async () => {
    if (!currentQuestion || bookmarkBusy) return;
    const questionId = currentQuestion.id;
    const alreadySaved = savedIds.has(questionId);
    setBookmarkBusy(true);
    setBookmarkError(false);
    try {
      if (alreadySaved) {
        await apiDelete<{ ok: boolean }>(`me/saved/${encodeURIComponent(questionId)}`);
      } else {
        await apiPost<{ ok: boolean }>("me/saved", { question_id: questionId });
      }
      setSavedIds((current) => {
        const next = new Set(current);
        if (alreadySaved) next.delete(questionId);
        else next.add(questionId);
        return next;
      });
    } catch {
      setBookmarkError(true);
    } finally {
      setBookmarkBusy(false);
    }
  };

  const toggleFullscreen = async () => {
    try {
      if (document.fullscreenElement) await document.exitFullscreen();
      else await document.documentElement.requestFullscreen();
    } catch {
      // Fullscreen is an optional browser capability; the learning flow remains usable.
    }
  };

  const modeLabel = (mode: SessionMode) => {
    const keys: Record<SessionMode, "modeVariant" | "modeExam" | "modePractice" | "modeMistakes"> = {
      variant: "modeVariant",
      exam: "modeExam",
      practice: "modePractice",
      mistakes: "modeMistakes",
    };
    return t(keys[mode]);
  };

  const localizedError = () => {
    if (!error) return t("genericError");
    if (error.code === "network_error") return t("networkError");
    if (error.code === "already_answered") return t("alreadyAnswered");
    if (error.code === "session_finished") return t("sessionFinished");
    return t("genericError");
  };

  if (loading && !session) {
    return (
      <main className="mx-auto flex min-h-[60vh] max-w-4xl items-center justify-center px-4 py-12">
        <div role="status" aria-live="polite" className="flex items-center gap-2 text-muted-foreground">
          <LoaderCircle className="h-5 w-5 animate-spin text-accent" aria-hidden="true" />
          {t("loading")}
        </div>
      </main>
    );
  }

  if (!session) {
    return (
      <main className="mx-auto max-w-xl px-4 py-12">
        <Card className="border-destructive/40 bg-destructive/5 p-8 text-center" role="alert">
          <AlertTriangle className="mx-auto h-10 w-10 text-destructive" aria-hidden="true" />
          <h1 className="mt-3 font-display text-xl font-bold">{t("errorTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {error ? localizedError() : t("notFound")}
          </p>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            {sessionId && (
              <Button variant="game" onClick={() => void loadSession(sessionId, locale)}>
                <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
                {t("retry")}
              </Button>
            )}
            <Button variant="outline" onClick={() => router.push(`/${locale}/tickets`)}>
              {t("backToTickets")}
            </Button>
          </div>
        </Card>
      </main>
    );
  }

  if (session.status !== "completed" && questions.length === 0) {
    return (
      <main className="mx-auto max-w-xl px-4 py-12">
        <Card className="border-accent/40 bg-accent/5 p-8 text-center">
          <AlertTriangle className="mx-auto h-10 w-10 text-accent" aria-hidden="true" />
          <h1 className="mt-3 font-display text-xl font-bold">{t("emptyTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t("emptyBody")}</p>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <Button variant="game" onClick={() => void loadSession(sessionId, locale)}>
              <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
              {t("retry")}
            </Button>
            <Button variant="outline" onClick={() => router.push(`/${locale}/tickets`)}>
              {t("backToTickets")}
            </Button>
          </div>
        </Card>
      </main>
    );
  }

  if (session.status === "completed") {
    const score = session.score ?? 0;
    const total = session.total ?? questions.length;
    const percentage = total > 0 ? Math.round((score / total) * 100) : 0;
    const passed = session.passed === true;

    return (
      <main className="mx-auto max-w-4xl space-y-6 px-4 py-10">
        <Card className={`p-8 text-center shadow-xl ${passed ? "border-success/40" : "border-border"}`}>
          <div
            className={`mx-auto flex h-20 w-20 items-center justify-center rounded-full border ${
              passed ? "border-success/40 bg-success/10" : "border-accent/30 bg-accent/10"
            }`}
          >
            {passed ? (
              <Award className="h-11 w-11 text-gold" aria-hidden="true" />
            ) : (
              <RefreshCw className="h-10 w-10 text-accent" aria-hidden="true" />
            )}
          </div>
          <h1 className="mt-5 font-display text-3xl font-extrabold">
            {passed ? t("passedTitle") : t("failedTitle")}
          </h1>
          <p className="mx-auto mt-2 max-w-xl text-sm text-muted-foreground">
            {passed ? t("passedBody") : t("failedBody")}
          </p>

          {session.stopped_reason && session.stopped_reason !== "completed" && (
            <p className="mx-auto mt-4 max-w-lg rounded-xl border border-gold/30 bg-gold/10 px-4 py-3 text-sm">
              {session.stopped_reason === "too_many_errors" ? t("stoppedTooMany") : t("stoppedTime")}
            </p>
          )}

          <dl className="mx-auto my-8 grid max-w-sm grid-cols-2 gap-4">
            <div className="rounded-2xl border border-border bg-background/50 p-4">
              <dt className="text-xs font-bold text-muted-foreground">{t("scoreLabel")}</dt>
              <dd className="font-display text-3xl font-black text-accent">
                {score} / {total}
              </dd>
            </div>
            <div className="rounded-2xl border border-border bg-background/50 p-4">
              <dt className="text-xs font-bold text-muted-foreground">{t("percentageLabel")}</dt>
              <dd className="font-display text-3xl font-black text-gold">{percentage}%</dd>
            </div>
          </dl>

          <div className="flex flex-wrap justify-center gap-3">
            <Button variant="game" size="lg" onClick={() => router.push(`/${locale}/tickets`)}>
              {t("backToTickets")}
            </Button>
            <Button variant="outline" size="lg" onClick={() => router.push(`/${locale}/dashboard`)}>
              {t("dashboard")}
            </Button>
          </div>
        </Card>

        {questions.length > 0 && (
          <section className="space-y-3" aria-labelledby="session-review-title">
            <h2 id="session-review-title" className="font-display text-xl font-bold">
              {t("reviewTitle")}
            </h2>
            {questions.map((question, index) => {
              const userAnswer = question.answers.find((answer) => answer.id === question.user_answer_id);
              const correctAnswer = question.answers.find(
                (answer) => answer.id === question.correct_answer_id
              );
              return (
                <Card key={question.id} className="p-5">
                  <div className="flex items-start gap-3">
                    {question.correct === true ? (
                      <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-success" aria-hidden="true" />
                    ) : (
                      <XCircle className="mt-0.5 h-5 w-5 shrink-0 text-danger" aria-hidden="true" />
                    )}
                    <div className="min-w-0 flex-1">
                      <h3 className="font-display font-bold">
                        {t("reviewQuestion", { number: index + 1 })}: {question.question}
                      </h3>
                      <dl className="mt-3 grid gap-2 text-sm sm:grid-cols-2">
                        <div>
                          <dt className="font-semibold text-muted-foreground">{t("yourAnswer")}</dt>
                          <dd>{userAnswer?.text ?? t("unanswered")}</dd>
                        </div>
                        <div>
                          <dt className="font-semibold text-muted-foreground">{t("correctAnswer")}</dt>
                          <dd>{correctAnswer?.text ?? t("notAvailable")}</dd>
                        </div>
                      </dl>
                      {question.explanation && question.explanation.blocks.length > 0 && (
                        <details className="mt-4 rounded-xl border border-accent/30 bg-accent/5 p-3">
                          <summary className="cursor-pointer text-sm font-bold text-accent">
                            {t("openExplanation")}
                          </summary>
                          <div className="mt-3 space-y-3 border-t border-accent/20 pt-3">
                            {question.explanation.blocks.map((block, blockIndex) => (
                              <p key={`${block.type}-${blockIndex}`} className="whitespace-pre-line text-sm leading-6">
                                {block.content}
                              </p>
                            ))}
                          </div>
                        </details>
                      )}
                    </div>
                  </div>
                </Card>
              );
            })}
          </section>
        )}
      </main>
    );
  }

  const currentSaved = currentQuestion ? savedIds.has(currentQuestion.id) : false;
  const canGoNext = currentAnswered && currentIndex < questions.length - 1;
  const isLast = currentIndex === questions.length - 1;

  return (
    <main className="mx-auto max-w-5xl space-y-5 px-4 py-6">
      <header className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-border bg-card/90 p-3 shadow-sm backdrop-blur-md">
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            aria-label={t("exit")}
            onClick={() => router.push(`/${locale}/dashboard`)}
          >
            <ChevronLeft className="h-4 w-4" aria-hidden="true" />
            <span className="hidden sm:inline">{t("exit")}</span>
          </Button>
          <span className="rounded-full border border-accent/30 bg-accent/10 px-3 py-1 text-xs font-bold text-accent">
            {modeLabel(session.mode)}
          </span>
        </div>

        {session.mode === "exam" && session.remaining_sec !== null && (
          <div className="flex items-center gap-2" aria-label={t("timeRemaining")}>
            <Clock3 className="h-4 w-4 text-gold" aria-hidden="true" />
            <CountdownTimer seconds={session.remaining_sec} onExpire={() => void handleFinish()} />
          </div>
        )}

        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={() => void toggleBookmark()}
            disabled={!currentQuestion || bookmarkBusy}
            aria-label={currentSaved ? t("removeBookmark") : t("addBookmark")}
            aria-pressed={currentSaved}
            className="flex h-11 w-11 items-center justify-center rounded-xl border border-border text-muted-foreground transition-colors hover:border-accent hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50"
          >
            {bookmarkBusy ? (
              <LoaderCircle className="h-4 w-4 animate-spin" aria-hidden="true" />
            ) : (
              <Bookmark className={`h-4 w-4 ${currentSaved ? "fill-accent text-accent" : ""}`} aria-hidden="true" />
            )}
          </button>

          <label className="sr-only" htmlFor="session-locale">
            {t("language")}
          </label>
          <select
            id="session-locale"
            value={locale}
            onChange={(event) => router.push(`/${event.target.value}/session/${sessionId}`)}
            className="h-11 rounded-xl border border-border bg-background px-2 text-xs font-bold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {locales.map((item) => (
              <option key={item} value={item}>
                {item === "uz-Latn" ? "O‘z" : item === "uz-Cyrl" ? "Ўз" : "Ru"}
              </option>
            ))}
          </select>

          <button
            type="button"
            onClick={() => void toggleFullscreen()}
            aria-label={isFullscreen ? t("exitFullscreen") : t("enterFullscreen")}
            className="flex h-11 w-11 items-center justify-center rounded-xl border border-border text-muted-foreground transition-colors hover:border-accent hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {isFullscreen ? (
              <Minimize2 className="h-4 w-4" aria-hidden="true" />
            ) : (
              <Expand className="h-4 w-4" aria-hidden="true" />
            )}
          </button>

          <Button variant="outline" size="sm" onClick={() => void handleFinish()} disabled={finishing || submitting}>
            {finishing ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" /> : null}
            {finishing ? t("finishing") : t("finish")}
          </Button>
        </div>
      </header>

      {(error || bookmarkError) && (
        <div
          role="alert"
          className="flex flex-col items-start justify-between gap-3 rounded-xl border border-destructive/40 bg-destructive/10 p-4 text-sm sm:flex-row sm:items-center"
        >
          <span>{bookmarkError ? t("bookmarkError") : localizedError()}</span>
          {pendingAnswer && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => void handleSelectAnswer(pendingAnswer.questionId, pendingAnswer.answerId)}
              disabled={submitting}
            >
              <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
              {t("retryAnswer")}
            </Button>
          )}
        </div>
      )}

      <nav
        className="flex gap-2 overflow-x-auto rounded-2xl border border-border bg-card p-3 shadow-sm"
        aria-label={t("questionNavigator")}
      >
        {questions.map((question, index) => {
          const isCurrent = index === currentIndex;
          const answered = hasAnswer(question);
          const status = isCurrent
            ? t("statusCurrent")
            : question.correct === true
              ? t("statusCorrect")
              : question.correct === false
                ? t("statusWrong")
                : answered
                  ? t("statusAnswered")
                  : t("statusUnanswered");
          const style = isCurrent
            ? "border-accent bg-accent text-white ring-2 ring-accent/30"
            : question.correct === true
              ? "border-success/50 bg-success/15 text-success"
              : question.correct === false
                ? "border-danger/50 bg-danger/15 text-danger"
                : answered
                  ? "border-accent/40 bg-accent/10 text-accent"
                  : "border-border bg-background text-muted-foreground";

          return (
            <button
              key={question.id}
              type="button"
              onClick={() => setCurrentIndex(index)}
              aria-current={isCurrent ? "step" : undefined}
              aria-label={t("questionNavLabel", { number: index + 1, status })}
              className={`relative flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border text-xs font-extrabold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${style}`}
            >
              {index + 1}
              {!isCurrent && question.correct === true && (
                <CheckCircle2 className="absolute -right-1 -top-1 h-3.5 w-3.5 fill-background" aria-hidden="true" />
              )}
              {!isCurrent && question.correct === false && (
                <XCircle className="absolute -right-1 -top-1 h-3.5 w-3.5 fill-background" aria-hidden="true" />
              )}
              {!isCurrent && answered && question.correct === undefined && (
                <Check className="absolute -right-1 -top-1 h-3.5 w-3.5 rounded-full bg-background" aria-hidden="true" />
              )}
            </button>
          );
        })}
      </nav>

      {currentQuestion && (
        <Card className="space-y-6 p-5 sm:p-7">
          <QuestionCard
            questionNumber={currentIndex + 1}
            totalQuestions={questions.length}
            questionText={currentQuestion.question}
            imageUrl={currentQuestion.image_url}
            onImageClick={
              currentQuestion.image_url ? () => setZoomImageUrl(currentQuestion.image_url ?? null) : undefined
            }
            explanation={currentQuestion.explanation}
          />

          <div className="space-y-3 border-t border-border pt-5">
            {currentQuestion.answers.map((answer, index) => (
              <AnswerOption
                key={answer.id}
                id={answer.id}
                index={index}
                text={answer.text}
                state={answerState(currentQuestion, answer.id)}
                disabled={currentAnswered || submitting}
                onSelect={(answerId) => void handleSelectAnswer(currentQuestion.id, answerId)}
              />
            ))}
          </div>

          {currentAnswered && currentQuestion.correct === undefined && (
            <p className="flex items-center gap-2 rounded-xl border border-accent/30 bg-accent/10 p-3 text-sm font-semibold text-accent">
              <CheckCircle2 className="h-4 w-4 shrink-0" aria-hidden="true" />
              {t("answerAccepted")}
            </p>
          )}
        </Card>
      )}

      <footer className="sticky bottom-3 flex items-center justify-between gap-3 rounded-2xl border border-border bg-card/95 p-3 shadow-xl backdrop-blur-md">
        <Button
          variant="outline"
          disabled={currentIndex === 0}
          onClick={() => setCurrentIndex((value) => Math.max(0, value - 1))}
        >
          <ChevronLeft className="mr-1 h-4 w-4" aria-hidden="true" />
          {t("previous")}
        </Button>

        {isLast ? (
          <Button
            variant="game"
            disabled={!currentAnswered || finishing || submitting}
            onClick={() => void handleFinish()}
          >
            {finishing ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" /> : null}
            {finishing ? t("finishing") : t("finish")}
          </Button>
        ) : (
          <Button
            variant="game"
            disabled={!canGoNext}
            onClick={() => setCurrentIndex((value) => Math.min(questions.length - 1, value + 1))}
          >
            {t("next")}
            <ChevronRight className="ml-1 h-4 w-4" aria-hidden="true" />
          </Button>
        )}
      </footer>

      {zoomImageUrl && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label={t("zoomDialog")}
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) setZoomImageUrl(null);
          }}
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/85 p-4 backdrop-blur-sm"
        >
          <div className="relative max-h-[90vh] max-w-5xl">
            {/* Dynamic media URL is served by the backend. */}
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={zoomImageUrl}
              alt={t("zoomedImageAlt")}
              className="max-h-[88vh] w-full rounded-2xl object-contain"
            />
            <button
              type="button"
              onClick={() => setZoomImageUrl(null)}
              aria-label={t("closeZoom")}
              className="absolute right-2 top-2 flex h-11 w-11 items-center justify-center rounded-full bg-slate-950/85 text-white shadow-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white"
            >
              <X className="h-5 w-5" aria-hidden="true" />
            </button>
          </div>
        </div>
      )}
    </main>
  );
}
