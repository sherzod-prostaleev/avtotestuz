import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { DangerConfirm } from "./danger-confirm";

describe("DangerConfirm", () => {
  it("disables confirm until phrase matches", () => {
    const onConfirm = vi.fn();
    render(
      <DangerConfirm
        open
        onOpenChange={() => {}}
        title="Xavfli amal"
        warnings={["Bu amalni bekor qilib bo‘lmaydi."]}
        confirmPhrase="O'CHIRISH"
        confirmPhraseLabel="Tasdiqlash uchun yozing:"
        confirmLabel="Davom etish"
        cancelLabel="Bekor qilish"
        onConfirm={onConfirm}
      />,
    );
    const btn = screen.getByRole("button", { name: "Davom etish" });
    expect(btn).toBeDisabled();
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "O'CHIRISH" } });
    expect(btn).toBeEnabled();
    fireEvent.click(btn);
    expect(onConfirm).toHaveBeenCalledWith("O'CHIRISH");
  });

  it("accepts an explicitly configured alternative phrase", () => {
    const onConfirm = vi.fn();
    render(
      <DangerConfirm
        open
        onOpenChange={() => {}}
        title="Xavfli amal"
        warnings={["Qaytarib bo‘lmaydi."]}
        confirmPhrase="+998901112233"
        confirmAlternatives={["DELETE"]}
        confirmPhraseLabel="Telefon yoki DELETE:"
        confirmLabel="O‘chirish"
        cancelLabel="Bekor qilish"
        onConfirm={onConfirm}
      />,
    );
    const btn = screen.getByRole("button", { name: "O‘chirish" });
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "DELETE" } });
    expect(btn).toBeEnabled();
    fireEvent.click(btn);
    expect(onConfirm).toHaveBeenCalledOnce();
    expect(onConfirm).toHaveBeenCalledWith("DELETE");
  });
});
