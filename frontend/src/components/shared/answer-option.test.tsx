import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { AnswerOption } from "./answer-option";

describe("AnswerOption", () => {
  it("shows a checkmark only in the correct state", () => {
    render(<AnswerOption shortcutLabel="F1" text="To'g'ridan burilish" state="correct" />);
    expect(screen.getByTestId("answer-correct-icon")).toBeInTheDocument();
  });

  it("shows an x-mark only in the incorrect state", () => {
    render(<AnswerOption shortcutLabel="F2" text="Chapga burilish" state="incorrect" />);
    expect(screen.getByTestId("answer-incorrect-icon")).toBeInTheDocument();
  });

  it("calls onSelect when clicked", () => {
    const onSelect = vi.fn();
    render(<AnswerOption shortcutLabel="F1" text="Variant" state="neutral" onSelect={onSelect} />);
    fireEvent.click(screen.getByRole("button"));
    expect(onSelect).toHaveBeenCalledOnce();
  });

  it("never exposes a way to read correctness in the hidden (exam) state", () => {
    render(<AnswerOption shortcutLabel="F1" text="Variant" state="hidden" />);
    expect(screen.queryByTestId("answer-correct-icon")).not.toBeInTheDocument();
    expect(screen.queryByTestId("answer-incorrect-icon")).not.toBeInTheDocument();
  });
});
