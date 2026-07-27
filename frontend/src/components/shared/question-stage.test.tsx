import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import { QuestionStage } from "./question-stage";
import type { SessionQuestionItem } from "@/hooks/use-session-engine";
import { QUESTION_IMAGE_PLACEHOLDER } from "@/lib/question-image";

function buildQuestion(overrides: Partial<SessionQuestionItem> = {}): SessionQuestionItem {
  return {
    id: "q1",
    question: "Chorrahada kim ustunlikka ega?",
    image_url: null,
    answers: [
      { id: "a1", text: "Birinchi javob" },
      { id: "a2", text: "Ikkinchi javob" },
      { id: "a3", text: "Uchinchi javob" },
    ],
    ...overrides,
  } as SessionQuestionItem;
}

function renderStage(question: SessionQuestionItem, props: Partial<React.ComponentProps<typeof QuestionStage>> = {}) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <QuestionStage
        question={question}
        questionNumber={1}
        totalQuestions={20}
        answered={false}
        disabled={false}
        onSelectAnswer={vi.fn()}
        onZoomImage={vi.fn()}
        onOpenExplanation={vi.fn()}
        {...props}
      />
    </NextIntlClientProvider>
  );
}

describe("QuestionStage", () => {
  it("uses the two-column layout when the question has an image", () => {
    renderStage(buildQuestion({ image_url: "https://media.example.test/q.webp" }));

    expect(screen.getByTestId("question-stage")).toHaveAttribute("data-layout", "two-column");
    expect(screen.getByRole("img", { name: "1-savol rasmi" })).toHaveAttribute(
      "src",
      "https://media.example.test/q.webp"
    );
  });

  it("shows the Driver Go cars placeholder when there is no image", () => {
    renderStage(buildQuestion({ image_url: null }));

    expect(screen.getByTestId("question-stage")).toHaveAttribute("data-layout", "two-column");
    expect(screen.getByRole("img", { name: "1-savol rasmi" })).toHaveAttribute(
      "src",
      QUESTION_IMAGE_PLACEHOLDER
    );
  });

  it("switches to compact density once a question carries five answers", () => {
    const question = buildQuestion({
      answers: [
        { id: "a1", text: "Bir" },
        { id: "a2", text: "Ikki" },
        { id: "a3", text: "Uch" },
        { id: "a4", text: "To'rt" },
        { id: "a5", text: "Besh" },
      ],
    } as Partial<SessionQuestionItem>);
    renderStage(question);

    expect(screen.getByTestId("question-stage")).toHaveAttribute("data-density", "compact");
  });

  it("keeps the default density for an ordinary question", () => {
    renderStage(buildQuestion());

    expect(screen.getByTestId("question-stage")).toHaveAttribute("data-density", "default");
  });

  it("switches to compact density when the wording is unusually long", () => {
    renderStage(
      buildQuestion({
        question: "S".repeat(300),
        answers: [
          { id: "a1", text: "J".repeat(120) },
          { id: "a2", text: "J".repeat(120) },
        ],
      } as Partial<SessionQuestionItem>)
    );

    expect(screen.getByTestId("question-stage")).toHaveAttribute("data-density", "compact");
  });

  it("keeps the explanation out of the answering flow and offers it as an action", () => {
    const onOpenExplanation = vi.fn();
    renderStage(
      buildQuestion({
        explanation: { blocks: [{ type: "muhim", content: "Asosiy qoida matni" }] },
      } as Partial<SessionQuestionItem>),
      { answered: true, onOpenExplanation }
    );

    expect(screen.queryByText("Asosiy qoida matni")).not.toBeInTheDocument();
    screen.getByRole("button", { name: "Ekspert tahlili" }).click();
    expect(onOpenExplanation).toHaveBeenCalledOnce();
  });

  it("does not offer the explanation action before the question is answered", () => {
    renderStage(
      buildQuestion({
        explanation: { blocks: [{ type: "muhim", content: "Asosiy qoida matni" }] },
      } as Partial<SessionQuestionItem>),
      { answered: false }
    );

    expect(screen.queryByRole("button", { name: "Ekspert tahlili" })).not.toBeInTheDocument();
  });

  it("keeps answer rows content-sized so options never crush or overlap", () => {
    renderStage(
      buildQuestion({
        answers: [
          {
            id: "a1",
            text: "Transport vositalaridan foydalanish va texnik holatiga javobgar mansabdor shaxslarning majburiyatlari",
          },
          { id: "a2", text: "Ikkinchi javob matni ham yetarlicha uzun bo'lishi mumkin" },
          { id: "a3", text: "Uchinchi variant" },
        ],
      } as Partial<SessionQuestionItem>)
    );

    const stage = screen.getByTestId("question-stage");
    expect(stage.className).toMatch(/\boverflow-hidden\b/);
    expect(stage.className).not.toMatch(/\boverflow-y-auto\b/);

    const list = screen.getByTestId("answer-list");
    const buttons = list.querySelectorAll("[data-answer-option]");
    expect(buttons).toHaveLength(3);
    buttons.forEach((button) => {
      expect(button.className).toMatch(/\bshrink-0\b/);
      expect(button.className).toMatch(/\bh-auto\b/);
    });
  });

  it("exposes a fit-scale attribute for viewport compaction", () => {
    renderStage(buildQuestion());
    const stage = screen.getByTestId("question-stage");
    expect(stage).toHaveAttribute("data-fit-scale");
  });
});
