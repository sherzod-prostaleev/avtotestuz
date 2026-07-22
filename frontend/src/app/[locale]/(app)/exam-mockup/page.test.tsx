import { render, screen, fireEvent } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import ExamMockupPage from "./page";

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ExamMockupPage />
    </NextIntlClientProvider>
  );
}

describe("ExamMockupPage", () => {
  it("shows no correct/incorrect feedback in the unanswered state", () => {
    renderWithIntl();
    expect(screen.queryByTestId("answer-correct-icon")).not.toBeInTheDocument();
    expect(screen.queryByTestId("answer-incorrect-icon")).not.toBeInTheDocument();
  });

  it("reveals the correct answer and explanation when switched to the correct state", () => {
    renderWithIntl();
    fireEvent.click(screen.getByRole("button", { name: "To'g'ri javob berilgan" }));
    expect(screen.getByTestId("answer-correct-icon")).toBeInTheDocument();
    expect(screen.getByText("MUHIM")).toBeInTheDocument();
  });

  it("never reveals correctness in the exam-hidden state, even after selecting an answer", () => {
    renderWithIntl();
    fireEvent.click(screen.getByRole("button", { name: "Imtihon rejimi (feedback yashirin)" }));
    expect(screen.queryByTestId("answer-correct-icon")).not.toBeInTheDocument();
    expect(screen.queryByTestId("answer-incorrect-icon")).not.toBeInTheDocument();
  });

  it("shows a gold timer normally and a pulsating red timer in the exam-hidden (low-time) state", () => {
    renderWithIntl();
    expect(screen.getByTestId("countdown-timer").className).toContain("text-gold");
    fireEvent.click(screen.getByRole("button", { name: "Imtihon rejimi (feedback yashirin)" }));
    expect(screen.getByTestId("countdown-timer").className).toContain("text-danger");
  });
});
