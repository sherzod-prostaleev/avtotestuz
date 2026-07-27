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

  it("does not dim disabled options (avoids submit flicker)", () => {
    render(<AnswerOption shortcutLabel="F1" text="Variant" state="selected" disabled />);
    expect(screen.getByRole("button").className).not.toMatch(/disabled:opacity/);
    expect(screen.getByRole("button").className).not.toMatch(/active:scale/);
  });

  it("never shrinks below content height (prevents overlapping answer text)", () => {
    render(
      <AnswerOption
        shortcutLabel="F1"
        text="Transport vositalaridan foydalanish va texnik holatiga javobgar mansabdor shaxslarning majburiyatlari"
        state="correct"
        dense
      />
    );
    const button = screen.getByRole("button");
    expect(button.className).toMatch(/\bshrink-0\b/);
    expect(button.className).toMatch(/\bh-auto\b/);
    expect(button.className).toMatch(/\boverflow-visible\b/);
  });

  it("lets long answer copy wrap inside the button", () => {
    render(
      <AnswerOption
        shortcutLabel="F2"
        text="Uzoq matn: chorrahada imtiyozga ega bo'lgan transport vositalarining harakatlanish tartibi"
        state="neutral"
        dense
      />
    );
    const label = screen.getByText(/Uzoq matn/);
    expect(label.className).toMatch(/\bbreak-words\b/);
    expect(label.className).toMatch(/\bmin-w-0\b/);
    expect(label.className).toMatch(/\bwhitespace-normal\b/);
  });

  it("applies wrong-answer pulse class only for wrong/incorrect states", () => {
    const { rerender } = render(<AnswerOption shortcutLabel="F1" text="Variant" state="selected" />);
    expect(screen.getByRole("button").className).not.toContain("answer-wrong-pulse");
    rerender(<AnswerOption shortcutLabel="F1" text="Variant" state="wrong" disabled />);
    expect(screen.getByRole("button").className).toContain("answer-wrong-pulse");
  });
});
