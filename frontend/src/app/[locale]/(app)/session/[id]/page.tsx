"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useLocale } from "next-intl";
import { useSessionEngine } from "@/hooks/use-session-engine";
import { QuestionCard } from "@/components/shared/question-card";
import { AnswerOption, AnswerState } from "@/components/shared/answer-option";
import { CountdownTimer } from "@/components/shared/countdown-timer";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Award, XCircle, ChevronLeft, ChevronRight, CheckCircle2, RotateCcw, X, ZoomIn } from "lucide-react";

export default function TestSessionPage() {
  const params = useParams();
  const router = useRouter();
  const locale = useLocale();
  const sessionId = params.id as string;

  const { session, loading, submitting, error, loadSession, submitAnswer, finishSession } =
    useSessionEngine(sessionId);

  const [currentIndex, setCurrentIndex] = useState<number>(0);
  const [selectedAnswers, setSelectedAnswers] = useState<Record<string, string>>({});
  const [zoomImageUrl, setZoomImageUrl] = useState<string | null>(null);

  useEffect(() => {
    if (sessionId) {
      void loadSession(sessionId);
    }
  }, [sessionId, loadSession]);

  // Keyboard shortcut listener (F1-F5)
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (!session || session.status === "completed") return;
      const currentQuestion = session.questions[currentIndex];
      if (!currentQuestion) return;

      const keys = ["F1", "F2", "F3", "F4", "F5"];
      const keyIndex = keys.indexOf(e.key);
      if (keyIndex !== -1 && currentQuestion.answers[keyIndex]) {
        e.preventDefault();
        const answer = currentQuestion.answers[keyIndex];
        void handleSelectAnswer(currentQuestion.id, answer.id);
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [session, currentIndex]);

  if (loading && !session) {
    return (
      <main className="mx-auto max-w-4xl px-4 py-12 text-center text-muted-foreground animate-pulse">
        Sessiya yuklanmoqda...
      </main>
    );
  }

  if (error || !session) {
    return (
      <main className="mx-auto max-w-xl px-4 py-12">
        <Card className="p-8 text-center border-destructive/40 bg-destructive/5">
          <p className="font-bold text-destructive">Xatolik yuz berdi</p>
          <p className="mt-2 text-sm text-muted-foreground">{error || "Sessiya topilmadi"}</p>
          <Button variant="game" className="mt-6" onClick={() => router.push(`/${locale}/tickets`)}>
            Biletlarga qaytish
          </Button>
        </Card>
      </main>
    );
  }

  const isCompleted = session.status === "completed";
  const questions = session.questions || [];
  const currentQuestion = questions[currentIndex];

  // Debug log to help diagnose empty questions
  if (typeof window !== "undefined" && questions.length === 0) {
    console.warn("[AvtoTest] Session loaded but questions array is empty. Raw session:", session);
  }

  // Show clear state when session exists but has no questions
  if (!isCompleted && questions.length === 0) {
    return (
      <main className="mx-auto max-w-xl px-4 py-12">
        <Card className="p-8 text-center border-accent/40 bg-accent/5">
          <p className="font-bold text-accent">Savollar yuklanmadi</p>
          <p className="mt-2 text-sm text-muted-foreground">
            Backend API'dan savollar kelmoqda, lekin ular bo'sh qaytdi. Iltimos, qayta urinib ko'ring yoki boshqa biletni tanlang.
          </p>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <Button variant="game" onClick={() => void loadSession(sessionId)}>
              Qayta yuklash
            </Button>
            <Button variant="outline" onClick={() => router.push(`/${locale}/tickets`)}>
              Biletlarga qaytish
            </Button>
          </div>
        </Card>
      </main>
    );
  }

  const handleSelectAnswer = async (questionId: string, answerId: string) => {
    if (isCompleted || submitting) return;
    setSelectedAnswers((prev) => ({ ...prev, [questionId]: answerId }));
    const updated = await submitAnswer(sessionId, questionId, answerId);
    if (updated?.status === "completed") {
      void loadSession(sessionId);
    }
  };

  const getAnswerState = (questionId: string, answerId: string): AnswerState => {
    if (!currentQuestion) return "neutral";
    const selected = selectedAnswers[questionId] || currentQuestion.user_answer_id;

    if (currentQuestion.is_correct !== undefined && currentQuestion.is_correct !== null) {
      if (currentQuestion.correct_answer_id === answerId) return "correct";
      if (selected === answerId && !currentQuestion.is_correct) return "wrong";
    }

    if (selected === answerId) return "selected";
    return "neutral";
  };

  // Render Completed Summary Screen
  if (isCompleted) {
    const score = session.score ?? 0;
    const total = questions.length || 20;
    const pct = Math.round((score / total) * 100);
    const passed = session.passed ?? score >= 18;
    const titleText = passed ? "Tabriklaymiz, imtihondan o'tdingiz!" : "Bu safar o'tolmadingiz";

    return (
      <main className="mx-auto max-w-3xl px-4 py-12">
        <Card className="p-8 text-center border-border bg-card shadow-2xl">
          <div className="mx-auto mb-4 flex h-20 w-20 items-center justify-center rounded-full bg-background border border-border">
            {passed ? (
              <Award className="h-12 w-12 text-gold animate-bounce" />
            ) : (
              <XCircle className="h-12 w-12 text-danger" />
            )}
          </div>

          <h1 className="font-display text-3xl font-extrabold">{titleText}</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {passed
              ? "Siz rasmiy imtihon talabiga javob berdingiz va ajoyib natija ko'rsatdingiz!"
              : "Xatolaringiz ustida ishlang va qayta urinib ko'ring!"}
          </p>

          <div className="my-8 flex justify-center gap-6">
            <div className="rounded-2xl border border-border p-4 w-32 bg-background/50">
              <span className="text-xs text-muted-foreground font-bold">Natija</span>
              <p className="font-display text-3xl font-black text-accent">{score} / {total}</p>
            </div>
            <div className="rounded-2xl border border-border p-4 w-32 bg-background/50">
              <span className="text-xs text-muted-foreground font-bold">Foiz</span>
              <p className="font-display text-3xl font-black text-gold">{pct}%</p>
            </div>
          </div>

          <div className="flex flex-wrap justify-center gap-4">
            <Button variant="game" size="lg" onClick={() => router.push(`/${locale}/tickets`)}>
              Biletlarga qaytish
            </Button>
            <Button variant="outline" size="lg" onClick={() => router.push(`/${locale}/dashboard`)}>
              Bosh sahifa
            </Button>
          </div>
        </Card>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-4xl px-4 py-8 space-y-6">
      {/* Top Header: Timer + Finish CTA */}
      <header className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="sm" onClick={() => router.push(`/${locale}/tickets`)}>
            <ChevronLeft className="mr-1 h-4 w-4" /> Chiqish
          </Button>
          <span className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
            {session.mode === "exam" ? "Rasmiy Imtihon" : "Test Yechish"}
          </span>
        </div>

        {session.mode === "exam" && session.remaining_sec !== null && (
          <CountdownTimer seconds={session.remaining_sec} onExpire={() => finishSession(sessionId)} />
        )}

        <Button variant="outline" size="sm" onClick={() => finishSession(sessionId)}>
          Tugatish
        </Button>
      </header>

      {/* 1..20 Question Navigator Bar */}
      <div className="flex flex-wrap gap-1.5 justify-center rounded-2xl border border-border bg-card p-3 shadow-sm">
        {questions.map((q, idx) => {
          const isCurrent = idx === currentIndex;
          const isAnswered = Boolean(q.user_answer_id || selectedAnswers[q.id]);
          const isCorrect = q.is_correct === true;
          const isWrong = q.is_correct === false;

          let btnStyle = "border-border bg-background text-muted-foreground hover:border-accent";
          if (isCurrent) btnStyle = "border-accent bg-accent text-white font-black ring-2 ring-accent/40";
          else if (isCorrect) btnStyle = "border-success bg-success/20 text-success font-bold";
          else if (isWrong) btnStyle = "border-danger bg-danger/20 text-danger font-bold";
          else if (isAnswered) btnStyle = "border-accent/40 bg-accent/10 text-accent font-bold";

          return (
            <button
              key={q.id}
              onClick={() => setCurrentIndex(idx)}
              className={`h-9 w-9 rounded-xl border text-xs font-bold transition-all ${btnStyle}`}
            >
              {idx + 1}
            </button>
          );
        })}
      </div>

      {/* Main Question Card & Answer Options */}
      {currentQuestion && (
        <Card className="p-6 space-y-6">
          <QuestionCard
            questionNumber={currentIndex + 1}
            totalQuestions={questions.length}
            questionText={currentQuestion.question}
            imageUrl={currentQuestion.image_url}
            onImageClick={() => currentQuestion.image_url && setZoomImageUrl(currentQuestion.image_url)}
            explanation={currentQuestion.explanation}
          />

          <div className="space-y-3 pt-2">
            {currentQuestion.answers.map((answer, aIdx) => (
              <AnswerOption
                key={answer.id}
                id={answer.id}
                index={aIdx}
                text={answer.text}
                state={getAnswerState(currentQuestion.id, answer.id)}
                onSelect={(answerId) => handleSelectAnswer(currentQuestion.id, answerId)}
              />
            ))}
          </div>
        </Card>
      )}

      {/* Bottom Nav Prev / Next */}
      <footer className="flex items-center justify-between">
        <Button
          variant="outline"
          disabled={currentIndex === 0}
          onClick={() => setCurrentIndex((prev) => Math.max(0, prev - 1))}
        >
          <ChevronLeft className="mr-1 h-4 w-4" /> Oldingisi
        </Button>

        <Button
          variant="game"
          disabled={currentIndex === questions.length - 1}
          onClick={() => setCurrentIndex((prev) => Math.min(questions.length - 1, prev + 1))}
        >
          Keyingisi <ChevronRight className="ml-1 h-4 w-4" />
        </Button>
      </footer>

      {/* Image Zoom Modal */}
      {zoomImageUrl && (
        <div
          onClick={() => setZoomImageUrl(null)}
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4 backdrop-blur-sm cursor-zoom-out"
        >
          <div className="relative max-w-4xl max-h-[90vh]">
            <img src={zoomImageUrl} alt="Kattalashtirilgan rasm" className="max-h-[85vh] w-full object-contain rounded-2xl" />
            <button
              onClick={() => setZoomImageUrl(null)}
              className="absolute -top-4 -right-4 rounded-full bg-white p-2 text-slate-900 shadow-xl hover:scale-110 transition-transform"
            >
              <X className="h-6 w-6" />
            </button>
          </div>
        </div>
      )}
    </main>
  );
}
