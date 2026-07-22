"use client";

import { useState } from "react";
import { QuestionCard } from "@/components/shared/question-card";
import { AnswerOption, type AnswerState } from "@/components/shared/answer-option";
import { demoQuestion } from "@/lib/mock-data";

export function DemoQuestionBlock() {
  const [selectedId, setSelectedId] = useState<string | null>(null);

  function stateFor(answerId: string): AnswerState {
    if (!selectedId) return "neutral";
    if (answerId === demoQuestion.correctAnswerId) return "correct";
    if (answerId === selectedId) return "incorrect";
    return "neutral";
  }

  return (
    <div className="mx-auto max-w-xl">
      <QuestionCard
        questionNumber={1}
        totalQuestions={1}
        text={demoQuestion.text}
        hasImage={demoQuestion.hasImage}
      />
      <div className="mt-4 flex flex-col gap-3">
        {demoQuestion.answers.map((answer) => (
          <AnswerOption
            key={answer.id}
            shortcutLabel={answer.shortcutLabel}
            text={answer.text}
            state={stateFor(answer.id)}
            onSelect={() => setSelectedId(answer.id)}
          />
        ))}
      </div>
    </div>
  );
}
