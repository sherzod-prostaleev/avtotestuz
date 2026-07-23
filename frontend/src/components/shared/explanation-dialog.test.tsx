import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, expect, it, vi } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import { ExplanationDialog } from "./explanation-dialog";

const explanation = {
  blocks: [
    { type: "muhim", content: "Asosiy qoida" },
    { type: "maslahat", content: "Qo'shimcha maslahat" },
  ],
};

function renderDialog(props: Partial<React.ComponentProps<typeof ExplanationDialog>> = {}) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ExplanationDialog
        open
        onClose={vi.fn()}
        questionNumber={1}
        questionText="Chorrahada kim ustunlikka ega?"
        imageUrl={null}
        explanation={explanation}
        {...props}
      />
    </NextIntlClientProvider>
  );
}

describe("ExplanationDialog", () => {
  it("renders nothing while closed", () => {
    renderDialog({ open: false });

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("shows every explanation block expanded, unlike the old progressive disclosure", () => {
    renderDialog();

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText("Asosiy qoida")).toBeVisible();
    expect(screen.getByText("Qo'shimcha maslahat")).toBeVisible();
  });

  it("keeps the question in view alongside the explanation", () => {
    renderDialog();

    expect(screen.getByText("Chorrahada kim ustunlikka ega?")).toBeInTheDocument();
  });

  it("renders the question image when one exists", () => {
    renderDialog({ imageUrl: "https://media.example.test/q.webp" });

    expect(screen.getByRole("img")).toHaveAttribute("src", "https://media.example.test/q.webp");
  });

  it("closes on the close button and on Escape", () => {
    const onClose = vi.fn();
    renderDialog({ onClose });

    fireEvent.click(screen.getByRole("button", { name: "Yopish" }));
    expect(onClose).toHaveBeenCalledOnce();

    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(2);
  });
});
