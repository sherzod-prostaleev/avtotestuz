"use client";

import { startTransition, useCallback, useEffect, useRef, useState } from "react";
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
import { type AnswerState } from "@/components/shared/answer-option";
import { ExplanationDialog } from "@/components/shared/explanation-dialog";
import { QuestionStage } from "@/components/shared/question-stage";
import { OfficialAvtotestExamView } from "@/components/exam/official-avtotest-exam-view";
import { ExamPassCelebration } from "@/components/exam/exam-pass-celebration";
import { BiletPraiseBanner } from "@/components/exam/bilet-praise-banner";
import { GrandMockCertificateDialog } from "@/components/mock/grand-mock-certificate-dialog";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { meetsExamPassThreshold } from "@/lib/celebration-confetti";
import { resolveQuestionImageUrl } from "@/lib/question-image";

/** Modes that share the strict timed/anti-cheat exam pipeline — timer,
 * answer redaction until finish, F-key exam UI. Currently "exam",
 * "grand_mock", and "placement" (mirrors backend session.IsExamLike). */
function isExamLikeMode(mode: SessionMode): boolean {
  return mode === "exam" || mode === "grand_mock" || mode === "placement";
}

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

function answerState(
  question: SessionQuestionItem,
  answerId: string,
  pending?: PendingAnswer | null
): AnswerState {
  if (question.correct_answer_id === answerId) return "correct";
  if (question.user_answer_id === answerId) {
    return question.correct === false ? "wrong" : "selected";
  }
  // Optimistic selection while the grade request is in flight — avoids the
  // neutral→dim→wrong flash that reads as a full-window flicker.
  if (pending && pending.questionId === question.id && pending.answerId === answerId) {
    return "selected";
  }
  return "neutral";
}

export interface TestSessionPageProps {
  // Reused as-is under the login-free kiosk
  // (frontend/src/app/[locale]/(kiosk)/station/session/[id]/page.tsx): a
  // walk-up student has no dashboard/tickets/practice/mistakes under the
  // learner's login-gated namespace to land on, so every navigation this
  // screen performs must stay under /station/... instead.
  kiosk?: boolean;
}

export default function TestSessionPage({ kiosk = false }: TestSessionPageProps = {}) {
  const params = useParams();
  const router = useRouter();
  const locale = useLocale();
  const t = useTranslations("Session");
  const sessionId = typeof params.id === "string" ? params.id : "";
  const exitHref = `/${locale}/${kiosk ? "station" : "dashboard"}`;
  const kioskTickets = `/${locale}/${kiosk ? "station/tickets" : "tickets"}`;
  const kioskPractice = `/${locale}/${kiosk ? "station/practice" : "practice"}`;
  const { session, loading, submitting, error, loadSession, submitAnswer, finishSession } =
    useSessionEngine(sessionId);

  const [currentIndex, setCurrentIndex] = useState(0);
  const [zoomImageUrl, setZoomImageUrl] = useState<string | null>(null);
  const [explanationOpen, setExplanationOpen] = useState(false);
  const [pendingAnswer, setPendingAnswer] = useState<PendingAnswer | null>(null);
  const [savedIds, setSavedIds] = useState<Set<string>>(new Set());
  const [bookmarkBusy, setBookmarkBusy] = useState(false);
  const [bookmarkError, setBookmarkError] = useState(false);
  const [finishing, setFinishing] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [certificateOpen, setCertificateOpen] = useState(false);
  const [examPassOpen, setExamPassOpen] = useState(false);
  const [biletPraiseOpen, setBiletPraiseOpen] = useState(false);
  const initializedSessionRef = useRef<string | null>(null);
  const viewedQuestionsRef = useRef<Set<string>>(new Set());
  const certificateShownForRef = useRef<string | null>(null);
  const examPassShownForRef = useRef<string | null>(null);
  const biletPraiseShownForRef = useRef<string | null>(null);
  const autoFinishAttemptedRef = useRef<string | null>(null);
  const questionShownAtRef = useRef(Date.now());
  const activeChipRef = useRef<HTMLButtonElement | null>(null);

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

  // The explanation belongs to one question; carrying it across navigation would show stale content.
  useEffect(() => {
    setExplanationOpen(false);
  }, [currentIndex]);

  // Keep the active chip in view while advancing through long bilets on mobile.
  useEffect(() => {
    activeChipRef.current?.scrollIntoView({
      behavior: "smooth",
      inline: "center",
      block: "nearest",
    });
  }, [currentIndex]);

  // Grand Mock's reward theater (confetti + certificate) is additive on top
  // of the shared exam-like result screen — shown once per passed session,
  // never for exam/practice/etc.
  useEffect(() => {
    if (
      session &&
      session.mode === "grand_mock" &&
      session.status === "completed" &&
      session.passed === true &&
      certificateShownForRef.current !== session.id
    ) {
      certificateShownForRef.current = session.id;
      setCertificateOpen(true);
    }
  }, [session]);

  // Official exam simulation win moment — once per passed session.
  useEffect(() => {
    if (
      session &&
      session.mode === "exam" &&
      session.status === "completed" &&
      session.passed === true &&
      examPassShownForRef.current !== session.id
    ) {
      examPassShownForRef.current = session.id;
      setExamPassOpen(true);
    }
  }, [session]);

  // Bilet praise when the learner hits the exam pass bar (≥18/20) — lighter
  // than the full exam salute.
  useEffect(() => {
    if (
      !session ||
      session.mode !== "variant" ||
      session.status !== "completed" ||
      biletPraiseShownForRef.current === session.id
    ) {
      return;
    }
    const score = session.score ?? 0;
    const total = session.total ?? session.questions.length;
    if (!meetsExamPassThreshold(score, total)) return;
    biletPraiseShownForRef.current = session.id;
    setBiletPraiseOpen(true);
  }, [session]);

  useEffect(() => {
    const onFullscreenChange = () => setIsFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener("fullscreenchange", onFullscreenChange);
    return () => document.removeEventListener("fullscreenchange", onFullscreenChange);
  }, []);

  const goToQuestion = useCallback((index: number) => {
    setCurrentIndex(index);
  }, []);

  const handleSelectAnswer = useCallback(
    async (questionId: string, answerId: string) => {
      const question = session?.questions.find((item) => item.id === questionId);
      if (!session || session.status !== "active" || !question || hasAnswer(question) || submitting) {
        return;
      }

      const latencyMs = Math.max(0, Date.now() - questionShownAtRef.current);
      setPendingAnswer({ questionId, answerId });
      const response = await submitAnswer(sessionId, questionId, answerId, {
        latencyMs,
      });
      if (!response) {
        // Keep pending for retry UI when the network/API call fails.
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
    if (!session || session.status !== "active" || finishing || submitting) return null;
    setFinishing(true);
    setPendingAnswer(null);
    try {
      const completed = await finishSession(sessionId);
      if (completed) {
        trackEvent("session_finish", {
          session_id: completed.id,
          mode: completed.mode,
          status: isExamLikeMode(completed.mode)
            ? completed.passed
              ? "passed"
              : "failed"
            : "completed",
          score: completed.score,
          total: completed.total,
          stopped_reason: completed.stopped_reason,
        });
      }
      return completed;
    } finally {
      setFinishing(false);
    }
  }, [finishSession, finishing, session, sessionId, submitting]);

  // Exam-like modes had no reachable finish control after the last answer, so
  // a clean pass could sit on the active UI until the timer expired. Auto-finish
  // once every question is recorded (manual Finish remains as a backup).
  useEffect(() => {
    if (!session || session.status !== "active" || finishing || submitting) return;
    if (!isExamLikeMode(session.mode)) return;
    const qs = session.questions ?? [];
    if (qs.length === 0 || !qs.every(hasAnswer)) return;
    if (autoFinishAttemptedRef.current === session.id) return;
    autoFinishAttemptedRef.current = session.id;
    void handleFinish().then((completed) => {
      if (!completed) autoFinishAttemptedRef.current = null;
    });
  }, [session, finishing, submitting, handleFinish]);

  const questions = session?.questions ?? [];
  const currentQuestion = questions[currentIndex];
  const currentAnswered = currentQuestion ? hasAnswer(currentQuestion) : false;

  useEffect(() => {
    questionShownAtRef.current = Date.now();
  }, [currentQuestion?.id]);

  useEffect(() => {
    if (!session || !currentQuestion || session.status !== "active") return;
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
      if (!session || session.status !== "active" || !currentQuestion) return;

      if (event.key === "ArrowLeft" && currentIndex > 0) {
        event.preventDefault();
        goToQuestion(currentIndex - 1);
        return;
      }
      if (event.key === "ArrowRight" && currentAnswered && currentIndex < questions.length - 1) {
        event.preventDefault();
        goToQuestion(currentIndex + 1);
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
    goToQuestion,
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
    const keys: Record<
      SessionMode,
      | "modeVariant"
      | "modeExam"
      | "modePractice"
      | "modeMistakes"
      | "modeGrandMock"
      | "modeReview"
      | "modePlacement"
    > = {
      variant: "modeVariant",
      exam: "modeExam",
      practice: "modePractice",
      mistakes: "modeMistakes",
      grand_mock: "modeGrandMock",
      review: "modeReview",
      placement: "modePlacement",
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
      <main className="page-shell-narrow flex min-h-[60vh] items-center justify-center">
        <div role="status" aria-live="polite" className="flex items-center gap-2 text-muted-foreground">
          <LoaderCircle className="h-5 w-5 animate-spin text-accent" aria-hidden="true" />
          {t("loading")}
        </div>
      </main>
    );
  }

  if (!session) {
    return (
      <main className="page-shell-narrow">
        <Card className="border-destructive/40 bg-destructive/5 p-6 text-center sm:p-8" role="alert">
          <AlertTriangle className="mx-auto h-10 w-10 text-destructive" aria-hidden="true" />
          <h1 className="mt-3 font-display text-xl font-bold">{t("errorTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {error ? localizedError() : t("notFound")}
          </p>
          <div className="sticky-cta-bar mt-6 flex flex-col gap-2 sm:flex-row sm:justify-center">
            {sessionId && (
              <Button variant="game" className="w-full sm:w-auto" onClick={() => void loadSession(sessionId, locale)}>
                <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
                {t("retry")}
              </Button>
            )}
            <Button variant="outline" className="w-full sm:w-auto" onClick={() => router.push(kioskTickets)}>
              {t("backToTickets")}
            </Button>
          </div>
        </Card>
      </main>
    );
  }

  if (session.status === "result_pending") {
    return (
      <main className="page-shell-narrow">
        <Card className="border-gold/40 bg-gold/5 p-6 text-center sm:p-8" role="status" aria-live="polite">
          <RefreshCw className="mx-auto h-10 w-10 text-gold" aria-hidden="true" />
          <h1 className="mt-3 font-display text-xl font-bold">{t("resultPendingTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t("resultPendingBody")}</p>
          <div className="sticky-cta-bar mt-6">
            <Button className="w-full sm:w-auto" variant="game" onClick={() => void loadSession(sessionId, locale)}>
              <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
              {t("reloadResult")}
            </Button>
          </div>
        </Card>
      </main>
    );
  }

  if (session.status === "active" && questions.length === 0) {
    return (
      <main className="page-shell-narrow">
        <Card className="border-accent/40 bg-accent/5 p-6 text-center sm:p-8">
          <AlertTriangle className="mx-auto h-10 w-10 text-accent" aria-hidden="true" />
          <h1 className="mt-3 font-display text-xl font-bold">{t("emptyTitle")}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{t("emptyBody")}</p>
          <div className="sticky-cta-bar mt-6 flex flex-col gap-2 sm:flex-row sm:justify-center">
            <Button variant="game" className="w-full sm:w-auto" onClick={() => void loadSession(sessionId, locale)}>
              <RefreshCw className="mr-2 h-4 w-4" aria-hidden="true" />
              {t("retry")}
            </Button>
            <Button variant="outline" className="w-full sm:w-auto" onClick={() => router.push(kioskTickets)}>
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
    const isExamResult = isExamLikeMode(session.mode);
    const positiveResult = isExamResult ? passed : true;
    const completedBodyKey =
      session.mode === "variant"
        ? "completedVariantBody"
        : session.mode === "practice"
          ? "completedPracticeBody"
          : "completedMistakesBody";
    const resultTitle = isExamResult ? (passed ? t("passedTitle") : t("failedTitle")) : t("completedTitle");
    const resultBody = isExamResult ? (passed ? t("passedBody") : t("failedBody")) : t(completedBodyKey);
    const primaryRoute =
      session.mode === "practice"
        ? kioskPractice
        // "mistakes" mode is never reached from the kiosk flow (the kiosk's
        // /station/practice never links to /mistakes), but fall back to the
        // station home rather than the login-gated /mistakes route just in
        // case a session somehow arrives in that mode.
        : session.mode === "mistakes"
          ? kiosk
            ? exitHref
            : `/${locale}/mistakes`
          : kioskTickets;
    const primaryLabel =
      session.mode === "practice"
        ? t("backToPractice")
        : session.mode === "mistakes"
          ? t("backToMistakes")
          : t("backToTickets");

    const wrongCount = questions.filter((q) => q.correct === false).length;

    return (
      <main className="page-shell-narrow space-y-5 sm:space-y-6">
        <Card className={`p-5 text-center sm:p-8 ${positiveResult ? "border-success/40" : "border-border"}`}>
          <div
            className={`mx-auto flex h-16 w-16 items-center justify-center rounded-2xl border sm:h-20 sm:w-20 sm:rounded-full ${
              positiveResult ? "border-success/40 bg-success/10" : "border-accent/30 bg-accent/10"
            }`}
          >
            {positiveResult ? (
              <Award className="h-9 w-9 text-gold sm:h-11 sm:w-11" aria-hidden="true" />
            ) : (
              <RefreshCw className="h-8 w-8 text-accent sm:h-10 sm:w-10" aria-hidden="true" />
            )}
          </div>
          <h1 className="mt-4 font-display text-2xl font-extrabold tracking-tight sm:mt-5 sm:text-3xl">{resultTitle}</h1>
          <p className="mx-auto mt-2 max-w-xl text-sm text-muted-foreground">{resultBody}</p>

          {isExamResult && session.stopped_reason && session.stopped_reason !== "completed" && (
            <p className="mx-auto mt-4 max-w-lg rounded-xl border border-gold/30 bg-gold/10 px-4 py-3 text-sm">
              {session.stopped_reason === "too_many_errors" ? t("stoppedTooMany") : t("stoppedTime")}
            </p>
          )}

          <dl className="mx-auto my-6 grid max-w-lg grid-cols-3 gap-2 sm:my-8 sm:gap-4">
            <div className="rounded-2xl border border-border bg-background p-3 sm:p-4">
              <dt className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground sm:text-xs">{t("scoreLabel")}</dt>
              <dd className="mt-1 font-display text-xl font-black tabular-nums text-accent sm:text-3xl">
                {score} / {total}
              </dd>
            </div>
            <div className="rounded-2xl border border-border bg-background p-3 sm:p-4">
              <dt className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground sm:text-xs">{t("percentageLabel")}</dt>
              <dd className="mt-1 font-display text-xl font-black tabular-nums text-gold sm:text-3xl">{percentage}%</dd>
            </div>
            <div className="rounded-2xl border border-border bg-background p-3 sm:p-4">
              <dt className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground sm:text-xs">{t("statusWrong")}</dt>
              <dd className="mt-1 font-display text-xl font-black tabular-nums text-danger sm:text-3xl">{wrongCount}</dd>
            </div>
          </dl>

          <div className="sticky-cta-bar flex flex-col gap-2 sm:flex-row sm:justify-center">
            <Button variant="game" size="lg" className="w-full sm:w-auto" onClick={() => router.push(primaryRoute)}>
              {primaryLabel}
            </Button>
            <Button variant="outline" size="lg" className="w-full sm:w-auto" onClick={() => router.push(exitHref)}>
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

        {session.mode === "grand_mock" && (
          <GrandMockCertificateDialog
            open={certificateOpen}
            onClose={() => setCertificateOpen(false)}
            score={score}
            total={total}
            shareCode={session.certificate_share_code}
          />
        )}

        {session.mode === "exam" && (
          <ExamPassCelebration
            open={examPassOpen}
            onClose={() => setExamPassOpen(false)}
            onDashboard={() => {
              setExamPassOpen(false);
              router.push(exitHref);
            }}
            score={score}
            total={total}
          />
        )}

        {session.mode === "variant" && (
          <BiletPraiseBanner
            open={biletPraiseOpen}
            onClose={() => setBiletPraiseOpen(false)}
            score={score}
            total={total}
          />
        )}
      </main>
    );
  }

  const currentSaved = currentQuestion ? savedIds.has(currentQuestion.id) : false;
  const canGoNext = currentAnswered && currentIndex < questions.length - 1;
  const isLast = currentIndex === questions.length - 1;

  if (isExamLikeMode(session.mode)) {
    return (
      <OfficialAvtotestExamView
        session={session}
        currentIndex={currentIndex}
        onSelectIndex={(index) => goToQuestion(index)}
        onSelectAnswer={(questionId, answerId) => void handleSelectAnswer(questionId, answerId)}
        onFinish={() => void handleFinish()}
        pendingAnswer={pendingAnswer}
        submitting={submitting}
        finishing={finishing}
        exitHref={exitHref}
      />
    );
  }

  return (
    <main
      className="session-shell flex flex-col gap-1 overflow-hidden bg-background px-2 pb-[max(0.35rem,env(safe-area-inset-bottom))] pt-[max(0.35rem,env(safe-area-inset-top))] sm:gap-3 sm:px-4 sm:py-3"
    >
      <header className="session-header flex shrink-0 items-center justify-between gap-1.5 rounded-xl border border-border bg-card px-1.5 py-1 sm:gap-3 sm:rounded-2xl sm:p-3">
        <div className="flex min-w-0 items-center gap-1.5">
          <Button
            variant="ghost"
            size="sm"
            className="h-9 min-h-9 w-9 px-0 sm:h-11 sm:min-h-11 sm:w-auto sm:px-4"
            aria-label={t("exit")}
            onClick={() => router.push(exitHref)}
          >
            <ChevronLeft className="h-4 w-4" aria-hidden="true" />
            <span className="hidden sm:inline">{t("exit")}</span>
          </Button>
          <span className="truncate rounded-md border border-accent/30 bg-accent/10 px-2 py-0.5 text-[10px] font-bold text-accent sm:px-3 sm:py-1 sm:text-xs">
            {modeLabel(session.mode)}
          </span>
        </div>

        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={() => void toggleBookmark()}
            disabled={!currentQuestion || bookmarkBusy}
            aria-label={currentSaved ? t("removeBookmark") : t("addBookmark")}
            aria-pressed={currentSaved}
            className="flex h-9 w-9 items-center justify-center rounded-lg border border-border text-muted-foreground transition-colors hover:border-accent hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-50 sm:h-11 sm:w-11 sm:rounded-xl"
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
            onChange={(event) => {
              const next = event.target.value;
              startTransition(() => {
                router.replace(`/${next}/${kiosk ? "station/session" : "session"}/${sessionId}`);
              });
            }}
            className="h-9 min-w-[3.75rem] rounded-lg border border-border bg-background px-1.5 text-[11px] font-bold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:h-11 sm:min-w-[4.5rem] sm:rounded-xl sm:px-2 sm:text-xs"
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
            className="hidden h-11 w-11 items-center justify-center rounded-xl border border-border text-muted-foreground transition-colors hover:border-accent hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:flex"
          >
            {isFullscreen ? (
              <Minimize2 className="h-4 w-4" aria-hidden="true" />
            ) : (
              <Expand className="h-4 w-4" aria-hidden="true" />
            )}
          </button>

          <Button
            variant="outline"
            size="sm"
            className="hidden sm:inline-flex"
            onClick={() => void handleFinish()}
            disabled={finishing || submitting}
          >
            {finishing ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" /> : null}
            {finishing ? t("finishing") : t("finish")}
          </Button>
        </div>
      </header>

      {(error || bookmarkError) && (
        <div
          role="alert"
          className="flex shrink-0 flex-col items-start justify-between gap-3 rounded-xl border border-destructive/40 bg-destructive/10 p-3 text-sm sm:flex-row sm:items-center"
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
        className="session-navigator chip-scroll shrink-0 rounded-xl border border-border bg-card p-1 sm:rounded-2xl sm:p-2"
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
            ? "border-accent bg-accent text-accent-foreground ring-2 ring-accent/30"
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
              ref={isCurrent ? activeChipRef : undefined}
              type="button"
              onClick={() => goToQuestion(index)}
              aria-current={isCurrent ? "step" : undefined}
              aria-label={t("questionNavLabel", { number: index + 1, status })}
              className={`relative flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border text-[11px] font-extrabold tabular-nums transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:h-11 sm:w-11 sm:rounded-xl sm:text-xs ${style}`}
            >
              {index + 1}
              {!isCurrent && question.correct === true && (
                <CheckCircle2 className="absolute -right-0.5 -top-0.5 h-3 w-3 fill-background sm:-right-1 sm:-top-1 sm:h-3.5 sm:w-3.5" aria-hidden="true" />
              )}
              {!isCurrent && question.correct === false && (
                <XCircle className="absolute -right-0.5 -top-0.5 h-3 w-3 fill-background sm:-right-1 sm:-top-1 sm:h-3.5 sm:w-3.5" aria-hidden="true" />
              )}
              {!isCurrent && answered && question.correct === undefined && (
                <Check className="absolute -right-0.5 -top-0.5 h-3 w-3 rounded-full bg-background sm:-right-1 sm:-top-1 sm:h-3.5 sm:w-3.5" aria-hidden="true" />
              )}
            </button>
          );
        })}
      </nav>

      {currentQuestion && (
        <Card className="session-content-card flex min-h-0 flex-1 flex-col gap-1 overflow-hidden p-1.5 sm:gap-3 sm:p-5">
          <div className="min-h-0 flex-1 overflow-hidden">
            <QuestionStage
              question={currentQuestion}
              questionNumber={currentIndex + 1}
              totalQuestions={questions.length}
              answered={currentAnswered}
              disabled={currentAnswered || submitting}
              answerStateFor={(answerId) => answerState(currentQuestion, answerId, pendingAnswer)}
              onSelectAnswer={(answerId) => void handleSelectAnswer(currentQuestion.id, answerId)}
              onZoomImage={() => setZoomImageUrl(resolveQuestionImageUrl(currentQuestion.image_url))}
              onOpenExplanation={() => setExplanationOpen(true)}
            />
          </div>

          {currentAnswered && currentQuestion.correct === undefined && (
            <p className="flex shrink-0 items-center gap-1.5 rounded-lg border border-accent/30 bg-accent/10 px-2 py-1 text-[11px] font-semibold text-accent sm:gap-2 sm:rounded-xl sm:p-2.5 sm:text-sm">
              <CheckCircle2 className="h-3.5 w-3.5 shrink-0 sm:h-4 sm:w-4" aria-hidden="true" />
              {t("answerAccepted")}
            </p>
          )}
        </Card>
      )}

      <footer className="session-actions flex shrink-0 items-center gap-1.5 rounded-xl border border-border bg-card p-1 sm:justify-between sm:gap-3 sm:rounded-2xl sm:p-2.5">
        <Button
          variant="outline"
          className="h-10 min-h-10 flex-1 sm:h-12 sm:min-h-12 sm:flex-none"
          disabled={currentIndex === 0}
          onClick={() => goToQuestion(Math.max(0, currentIndex - 1))}
        >
          <ChevronLeft className="mr-1 h-4 w-4" aria-hidden="true" />
          <span className="hidden xs:inline sm:inline">{t("previous")}</span>
        </Button>

        {isLast ? (
          <Button
            variant="game"
            className="h-10 min-h-10 flex-[1.4] sm:h-12 sm:min-h-12 sm:flex-none"
            disabled={!currentAnswered || finishing || submitting}
            onClick={() => void handleFinish()}
          >
            {finishing ? <LoaderCircle className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" /> : null}
            {finishing ? t("finishing") : t("finish")}
          </Button>
        ) : (
          <Button
            variant="game"
            className="h-10 min-h-10 flex-[1.4] sm:h-12 sm:min-h-12 sm:flex-none"
            disabled={!canGoNext}
            onClick={() => goToQuestion(Math.min(questions.length - 1, currentIndex + 1))}
          >
            {t("next")}
            <ChevronRight className="ml-1 h-4 w-4" aria-hidden="true" />
          </Button>
        )}
      </footer>

      {currentQuestion && (
        <ExplanationDialog
          open={explanationOpen}
          onClose={() => setExplanationOpen(false)}
          questionNumber={currentIndex + 1}
          questionText={currentQuestion.question}
          imageUrl={currentQuestion.image_url}
          explanation={currentQuestion.explanation ?? null}
        />
      )}

      {zoomImageUrl && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label={t("zoomDialog")}
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) setZoomImageUrl(null);
          }}
          className="fixed inset-0 z-50 flex items-end justify-center bg-black/85 p-0 sm:items-center sm:p-4"
        >
          <div className="session-zoom-panel relative w-full max-w-5xl rounded-t-3xl bg-card p-3 sm:rounded-2xl sm:bg-transparent sm:p-0">
            {/* Dynamic media URL is served by the backend. */}
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={zoomImageUrl}
              alt={t("zoomedImageAlt")}
              className="session-zoom-image w-full rounded-2xl object-contain"
            />
            <button
              type="button"
              onClick={() => setZoomImageUrl(null)}
              aria-label={t("closeZoom")}
              className="absolute right-3 top-3 flex h-11 w-11 items-center justify-center rounded-full border border-border bg-card text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:right-2 sm:top-2 sm:border-0 sm:bg-foreground/90 sm:text-background"
            >
              <X className="h-5 w-5" aria-hidden="true" />
            </button>
          </div>
        </div>
      )}
    </main>
  );
}
