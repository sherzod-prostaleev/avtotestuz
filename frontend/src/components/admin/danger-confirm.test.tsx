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
    expect(onConfirm).toHaveBeenCalled();
  });
});
