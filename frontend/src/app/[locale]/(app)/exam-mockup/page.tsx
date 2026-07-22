"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { QuestionCard } from "@/components/shared/question-card";
import { AnswerOption, type AnswerState } from "@/components/shared/answer-option";
import { CountdownTimer } from "@/components/shared/countdown-timer";
import { Button } from "@/components/ui/button";
import { mockExamQuestions } from "@/lib/mock-data";

type Mode = "unanswered" | "correct" | "incorrect" | "exam-hidden";

export default function ExamMockupPage() {
  const t = useTranslations("ExamMockup");
  const [mode, setMode] = useState<Mode>("unanswered");
  const question = mockExamQuestions[0];
  const wrongAnswerId = question.answers.find((a) => a.id !== question.correctAnswerId)!.id;
  const selectedAnswerId =
    mode === "correct" ? question.correctAnswerId : mode === "incorrect" ? wrongAnswerId : null;

  const modeLabels: Record<Mode, string> = {
    unanswered: t("modeUnanswered"),
    correct: t("modeCorrect"),
    incorrect: t("modeIncorrect"),
    "exam-hidden": t("modeExamHidden"),
  };

  function stateFor(answerId: string): AnswerState {
    if (mode === "unanswered") return "neutral";
    if (mode === "exam-hidden") return answerId === selectedAnswerId ? "selected" : "neutral";
    if (answerId === question.correctAnswerId) return "correct";
    if (answerId === selectedAnswerId) return "incorrect";
    return "neutral";
  }

  return (
    <main className="mx-auto max-w-2xl px-4 py-8">
      {/* Mockup-only tooling: Phase B3 replaces this with real session state. */}
      <div className="mb-4 flex flex-wrap gap-2" role="group" aria-label="Mockup holatini tanlash">
        {(Object.keys(modeLabels) as Mode[]).map((m) => (
          <Button key={m} size="sm" variant={m === mode ? "default" : "outline"} onClick={() => setMode(m)}>
            {modeLabels[m]}
          </Button>
        ))}
      </div>

      <div className="mb-4 flex items-center justify-between">
        <span className="text-sm text-muted-foreground">{t("positionLabel", { n: 1, total: 20 })}</span>
        <CountdownTimer remainingSeconds={mode === "exam-hidden" ? 45 : 900} />
      </div>

      <QuestionCard questionNumber={1} totalQuestions={20} text={question.text} hasImage={question.hasImage} />

      <div className="mt-4 flex flex-col gap-3">
        {question.answers.map((a) => (
          <AnswerOption key={a.id} shortcutLabel={a.shortcutLabel} text={a.text} state={stateFor(a.id)} />
        ))}
      </div>

      {(mode === "correct" || mode === "incorrect") && (
        <div className="mt-4 rounded-lg border border-gold bg-gold/10 p-4">
          <p className="font-display font-bold text-gold">MUHIM</p>
          <p className="mt-1 text-sm">
            Svetofor ishlamagan chorrahada — YHQning tegishli qoidasiga ko&apos;ra o&apos;ngdan kelayotgan
            transport vositasi ustunlikka ega bo&apos;ladi.
          </p>
        </div>
      )}
    </main>
  );
}
