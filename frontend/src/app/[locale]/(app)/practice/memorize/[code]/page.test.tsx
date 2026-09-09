import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../../../messages/uz-Latn.json";
import MemorizePage from "./page";
import { useMemorize } from "@/hooks/use-memorize";
import type { SessionQuestionItem } from "@/hooks/use-session-engine";

const navigation = vi.hoisted(() => ({ push: vi.fn() }));

vi.mock("next/navigation", () => ({
  useParams: () => ({ code: "signs" }),
  useRouter: () => ({ push: navigation.push }),
}));

vi.mock("@/hooks/use-memorize", () => ({ useMemorize: vi.fn() }));

function question(overrides: Partial<SessionQuestionItem> = {}): SessionQuestionItem {
  return {
    id: "q-1",
    question: "Qaysi belgi to'xtashni taqiqlaydi?",
    image_url: null,
    answers: [
      { id: "a-1", text: "3.27 belgisi" },
      { id: "a-2", text: "3.28 belgisi" },
    ],
    position: 1,
    answered: true,
    user_answer_id: null,
    correct_answer_id: "a-2",
    explanation: null,
    ...overrides,
  };
}

function renderPage(kiosk = false) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <MemorizePage kiosk={kiosk} />
    </NextIntlClientProvider>
  );
}

const mockUseMemorize = vi.mocked(useMemorize);

describe("MemorizePage", () => {
  beforeEach(() => {
    navigation.push.mockReset();
    mockUseMemorize.mockReset();
  });

  it("shows a loading state while the topic is fetched", () => {
    mockUseMemorize.mockReturnValue({ questions: [], loading: true, error: null });
    renderPage();
    expect(screen.getByText(messages.Memorize.loading)).toBeInTheDocument();
  });

  it("marks the correct answer from the very first render, with no click needed", async () => {
    mockUseMemorize.mockReturnValue({ questions: [question()], loading: false, error: null });
    renderPage();

    const correctOption = (await screen.findByText("3.28 belgisi")).closest("button")!;
    expect(correctOption.querySelector('[data-testid="answer-correct-icon"]')).toBeTruthy();
  });

  it("advances with Keyingi and shows the finished screen after the last question", async () => {
    mockUseMemorize.mockReturnValue({
      questions: [question({ id: "q-1" }), question({ id: "q-2", correct_answer_id: "a-1" })],
      loading: false,
      error: null,
    });
    renderPage();

    expect(await screen.findByText("Savol 1 / 2")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Keyingisi/ }));
    expect(await screen.findByText("Savol 2 / 2")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Keyingisi/ }));
    expect(await screen.findByText(messages.Memorize.finishedTitle)).toBeInTheDocument();
  });

  it("sends a non-VIP user to premium on vip_required", () => {
    mockUseMemorize.mockReturnValue({
      questions: [],
      loading: false,
      error: { code: "vip_required", message: "active entitlement required" },
    });
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: messages.SessionStart.goToPremium }));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/premium");
  });

  it("sends a kiosk vip_required user back to the station, never to premium", () => {
    mockUseMemorize.mockReturnValue({
      questions: [],
      loading: false,
      error: { code: "vip_required", message: "active entitlement required" },
    });
    renderPage(true);

    expect(
      screen.queryByRole("button", { name: messages.SessionStart.goToPremium })
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: messages.SessionStart.backToStation }));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/station");
  });
});
