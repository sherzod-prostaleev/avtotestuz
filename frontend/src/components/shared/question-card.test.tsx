import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import { QuestionCard } from "./question-card";

function renderCard(ui: React.ReactElement) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      {ui}
    </NextIntlClientProvider>
  );
}

describe("QuestionCard", () => {
  it("renders the localized question position and text", () => {
    renderCard(
      <QuestionCard questionNumber={3} totalQuestions={20} text="Chorrahada kim ustunlikka ega?" />
    );

    expect(screen.getByText("Savol 3 / 20")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Chorrahada kim ustunlikka ega?" })).toBeInTheDocument();
  });

  it("renders only a real image and makes zoom keyboard accessible", () => {
    const onZoom = vi.fn();
    renderCard(
      <QuestionCard
        questionNumber={1}
        totalQuestions={20}
        text="Savol matni"
        imageUrl="https://media.example.test/question.webp"
        onImageClick={onZoom}
      />
    );

    expect(screen.getByRole("img", { name: "1-savol rasmi" })).toHaveAttribute(
      "src",
      "https://media.example.test/question.webp"
    );
    fireEvent.click(screen.getByRole("button", { name: "Rasmni kattalashtirish" }));
    expect(onZoom).toHaveBeenCalledOnce();
  });

  it("does not fabricate a placeholder when an image URL is unavailable", () => {
    renderCard(<QuestionCard questionNumber={1} totalQuestions={20} text="Savol matni" hasImage />);

    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    expect(screen.queryByText("Savol rasmi")).not.toBeInTheDocument();
  });

});
