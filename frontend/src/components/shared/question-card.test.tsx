import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { QuestionCard } from "./question-card";

describe("QuestionCard", () => {
  it("renders the question position and text", () => {
    render(<QuestionCard questionNumber={3} totalQuestions={20} text="Chorrahada kim ustunlikka ega?" />);
    expect(screen.getByText("Savol 3 / 20")).toBeInTheDocument();
    expect(screen.getByText("Chorrahada kim ustunlikka ega?")).toBeInTheDocument();
  });

  it("shows an image placeholder only when hasImage is true", () => {
    const { rerender } = render(
      <QuestionCard questionNumber={1} totalQuestions={20} text="Savol matni" hasImage={false} />
    );
    expect(screen.queryByText("Savol rasmi")).not.toBeInTheDocument();
    rerender(<QuestionCard questionNumber={1} totalQuestions={20} text="Savol matni" hasImage />);
    expect(screen.getByText("Savol rasmi")).toBeInTheDocument();
  });
});
