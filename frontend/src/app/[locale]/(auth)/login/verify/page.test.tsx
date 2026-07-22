import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, afterEach } from "vitest";
import messages from "../../../../../../messages/uz-Latn.json";
import VerifyPage from "./page";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
  useSearchParams: () => new URLSearchParams("phone=901112233"),
}));

afterEach(() => {
  vi.unstubAllGlobals();
  pushMock.mockClear();
  vi.useRealTimers();
});

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <VerifyPage />
    </NextIntlClientProvider>
  );
}

function digitBoxes() {
  return screen.getAllByRole("textbox");
}

describe("VerifyPage", () => {
  it("auto-advances focus to the next digit box as each digit is typed", () => {
    renderWithIntl();
    const boxes = digitBoxes();
    fireEvent.change(boxes[0], { target: { value: "1" } });
    expect(boxes[1]).toHaveFocus();
  });

  it("auto-submits and navigates to /dashboard once all 6 digits are entered correctly", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { ok: true } }), { status: 200 }))
    );
    renderWithIntl();
    const boxes = digitBoxes();
    "123456".split("").forEach((digit, i) => fireEvent.change(boxes[i], { target: { value: digit } }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/dashboard"));
  });

  it("shows a translated error and does not navigate on an invalid code", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: "invalid_code" } }), { status: 400 }))
    );
    renderWithIntl();
    const boxes = digitBoxes();
    "000000".split("").forEach((digit, i) => fireEvent.change(boxes[i], { target: { value: digit } }));

    await waitFor(() => expect(screen.getByText("Kod noto'g'ri")).toBeInTheDocument());
    expect(pushMock).not.toHaveBeenCalled();
  });

  it("disables resend during the 60s cooldown and re-enables after it elapses", () => {
    vi.useFakeTimers();
    renderWithIntl();
    const resendButton = screen.getByRole("button");
    expect(resendButton).toBeDisabled();
    act(() => {
      vi.advanceTimersByTime(60_000);
    });
    expect(resendButton).not.toBeDisabled();
  });
});
