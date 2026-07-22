import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../messages/uz-Latn.json";
import { DemoQuestionBlock } from "./demo-question-block";

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function renderDemo() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <DemoQuestionBlock />
    </NextIntlClientProvider>
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("DemoQuestionBlock", () => {
  it("loads a correctness-free question and grades only through the server", async () => {
    let resolveGrade!: (response: Response) => void;
    const pendingGrade = new Promise<Response>((resolve) => {
      resolveGrade = resolve;
    });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            id: "question-1",
            text: "Real demo savoli",
            image_url: null,
            answers: [
              { id: "answer-1", position: 1, text: "Birinchi javob", image_url: null },
              { id: "answer-2", position: 2, text: "Ikkinchi javob", image_url: null },
            ],
          },
        })
      )
      .mockReturnValueOnce(pendingGrade);
    vi.stubGlobal("fetch", fetchMock);

    renderDemo();
    const firstAnswer = await screen.findByRole("button", { name: /F1.*Birinchi javob/i });
    expect(screen.queryByTestId("answer-correct-icon")).not.toBeInTheDocument();

    fireEvent.click(firstAnswer);
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/proxy/demo/answer",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ question_id: "question-1", answer_id: "answer-1" }),
      })
    );
    expect(screen.queryByTestId("answer-correct-icon")).not.toBeInTheDocument();

    await act(async () => {
      resolveGrade(
        jsonResponse({ data: { correct: false, correct_answer_id: "answer-2" } })
      );
      await pendingGrade;
    });

    await waitFor(() => expect(screen.getByTestId("answer-correct-icon")).toBeInTheDocument());
    expect(screen.getByTestId("answer-incorrect-icon")).toBeInTheDocument();
  });

  it("offers an honest retry state when the real demo is unavailable", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ error: { code: "not_found", message: "empty" } }, 404))
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            id: "question-2",
            text: "Qayta yuklangan savol",
            image_url: null,
            answers: [{ id: "answer-3", position: 1, text: "Javob", image_url: null }],
          },
        })
      );
    vi.stubGlobal("fetch", fetchMock);

    renderDemo();
    expect(await screen.findByRole("alert")).toHaveTextContent("Demo savolini yuklab bo'lmadi.");
    fireEvent.click(screen.getByRole("button", { name: "Qayta urinish" }));
    expect(await screen.findByText("Qayta yuklangan savol")).toBeInTheDocument();
  });
});
