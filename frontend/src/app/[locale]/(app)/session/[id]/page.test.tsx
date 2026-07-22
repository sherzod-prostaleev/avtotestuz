import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../../messages/uz-Latn.json";
import SessionPage from "./page";
import * as useSessionEngineModule from "@/hooks/use-session-engine";

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "sess-123" }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <SessionPage />
    </NextIntlClientProvider>
  );
}

describe("SessionPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders active session questions and options", () => {
    vi.spyOn(useSessionEngineModule, "useSessionEngine").mockReturnValue({
      session: {
        id: "sess-123",
        mode: "variant",
        time_limit_sec: null,
        remaining_sec: null,
        status: "active",
        questions: [
          {
            id: "q-1",
            question: "Qaysi belgi to'xtashni taqiqlaydi?",
            image_url: null,
            answers: [
              { id: "a1", text: "3.27 belgisi" },
              { id: "a2", text: "3.28 belgisi" },
            ],
            user_answer_id: null,
          },
        ],
        score: null,
        passed: null,
        completed_at: null,
      },
      loading: false,
      submitting: false,
      error: null,
      loadSession: vi.fn(),
      startSession: vi.fn(),
      submitAnswer: vi.fn(),
      finishSession: vi.fn(),
    });

    renderWithIntl();

    expect(screen.getByText("Qaysi belgi to'xtashni taqiqlaydi?")).toBeInTheDocument();
    expect(screen.getByText("3.27 belgisi")).toBeInTheDocument();
    expect(screen.getByText("3.28 belgisi")).toBeInTheDocument();
  });

  it("renders summary screen when session is completed", () => {
    vi.spyOn(useSessionEngineModule, "useSessionEngine").mockReturnValue({
      session: {
        id: "sess-123",
        mode: "exam",
        time_limit_sec: 1500,
        remaining_sec: 200,
        status: "completed",
        questions: [],
        score: 19,
        passed: true,
        completed_at: "2026-07-22T12:00:00Z",
      },
      loading: false,
      submitting: false,
      error: null,
      loadSession: vi.fn(),
      startSession: vi.fn(),
      submitAnswer: vi.fn(),
      finishSession: vi.fn(),
    });

    renderWithIntl();

    expect(screen.getByText("Tabriklaymiz, imtihondan o'tdingiz!")).toBeInTheDocument();
    expect(screen.getByText("19 / 20")).toBeInTheDocument();
  });
});
