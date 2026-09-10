import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../../../messages/uz-Latn.json";
import MemorizePage from "./page";
import { useMemorize } from "@/hooks/use-memorize";
import type { SessionQuestionItem } from "@/hooks/use-session-engine";
import { SESSION_ORIGIN_KEY } from "@/lib/session-origin";

const navigation = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));

vi.mock("next/navigation", () => ({
  useParams: () => ({ code: "signs" }),
  useRouter: () => ({ push: navigation.push, replace: navigation.replace }),
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
    window.sessionStorage.clear();
    navigation.push.mockReset();
    navigation.replace.mockReset();
    mockUseMemorize.mockReset();
  });

  it("exits back to the hub the learner opened Yodlash from", async () => {
    window.sessionStorage.setItem(SESSION_ORIGIN_KEY, "/uz-Latn/dashboard");
    mockUseMemorize.mockReturnValue({ questions: [question()], loading: false, error: null });
    renderPage();

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Chiqish" })).toBeInTheDocument()
    );
    fireEvent.click(screen.getByRole("button", { name: "Chiqish" }));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/dashboard");
  });

  it("falls back to the topic list when the tab remembers no hub", async () => {
    mockUseMemorize.mockReturnValue({ questions: [question()], loading: false, error: null });
    renderPage();

    fireEvent.click(await screen.findByRole("button", { name: "Chiqish" }));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/practice");
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

  // The whole point of the (session)-group move: a phone has to reach every
  // question the way the live test screen does, from the numbered chips, not
  // only through Keyingisi.
  it("renders a numbered chip per question and jumps straight to the tapped one", async () => {
    mockUseMemorize.mockReturnValue({
      questions: [
        question({ id: "q-1" }),
        question({ id: "q-2", correct_answer_id: "a-1" }),
        question({ id: "q-3", correct_answer_id: "a-1" }),
      ],
      loading: false,
      error: null,
    });
    renderPage();

    const navigator = await screen.findByRole("navigation", {
      name: messages.Session.questionNavigator,
    });
    expect(within(navigator).getAllByRole("button")).toHaveLength(3);

    fireEvent.click(within(navigator).getByRole("button", { name: /^3-savol/ }));
    expect(await screen.findByText("Savol 3 / 3")).toBeInTheDocument();
  });

  // A classroom PC is driven from the keyboard, not by tapping chips: the live
  // test screen walks the list with the arrow keys, and Yodlash has to as well.
  it("walks the topic with the left and right arrow keys", async () => {
    mockUseMemorize.mockReturnValue({
      questions: [
        question({ id: "q-1" }),
        question({ id: "q-2", correct_answer_id: "a-1" }),
        question({ id: "q-3", correct_answer_id: "a-1" }),
      ],
      loading: false,
      error: null,
    });
    renderPage();

    expect(await screen.findByText("Savol 1 / 3")).toBeInTheDocument();

    fireEvent.keyDown(window, { key: "ArrowRight" });
    expect(await screen.findByText("Savol 2 / 3")).toBeInTheDocument();

    fireEvent.keyDown(window, { key: "ArrowRight" });
    expect(await screen.findByText("Savol 3 / 3")).toBeInTheDocument();

    fireEvent.keyDown(window, { key: "ArrowLeft" });
    expect(await screen.findByText("Savol 2 / 3")).toBeInTheDocument();
  });

  // The arrows walk questions; they must never run off either end — least of
  // all into the finished screen, which Keyingisi alone is allowed to open.
  it("keeps the arrow keys inside the topic at both ends", async () => {
    mockUseMemorize.mockReturnValue({
      questions: [question({ id: "q-1" }), question({ id: "q-2", correct_answer_id: "a-1" })],
      loading: false,
      error: null,
    });
    renderPage();

    expect(await screen.findByText("Savol 1 / 2")).toBeInTheDocument();
    fireEvent.keyDown(window, { key: "ArrowLeft" });
    expect(await screen.findByText("Savol 1 / 2")).toBeInTheDocument();

    fireEvent.keyDown(window, { key: "ArrowRight" });
    expect(await screen.findByText("Savol 2 / 2")).toBeInTheDocument();
    fireEvent.keyDown(window, { key: "ArrowRight" });
    expect(await screen.findByText("Savol 2 / 2")).toBeInTheDocument();
    expect(screen.queryByText(messages.Memorize.finishedTitle)).not.toBeInTheDocument();
  });

  it("leaves the arrow keys to the browser while a dialog is open", async () => {
    mockUseMemorize.mockReturnValue({
      questions: [question({ id: "q-1" }), question({ id: "q-2", correct_answer_id: "a-1" })],
      loading: false,
      error: null,
    });
    renderPage();

    fireEvent.click(await screen.findByRole("button", { name: messages.Session.zoomImage }));
    expect(await screen.findByRole("dialog", { name: messages.Session.zoomDialog })).toBeInTheDocument();

    fireEvent.keyDown(window, { key: "ArrowRight" });
    expect(await screen.findByText("Savol 1 / 2")).toBeInTheDocument();

    // Escape closes the zoom, exactly like the live test screen.
    fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() =>
      expect(screen.queryByRole("dialog", { name: messages.Session.zoomDialog })).not.toBeInTheDocument()
    );
    fireEvent.keyDown(window, { key: "ArrowRight" });
    expect(await screen.findByText("Savol 2 / 2")).toBeInTheDocument();
  });

  // On a kiosk this is the only language control on the screen — KioskChrome's
  // floating bar steps aside for Yodlash rather than covering its header.
  it("switches language from the header without leaving the topic", async () => {
    mockUseMemorize.mockReturnValue({ questions: [question()], loading: false, error: null });
    renderPage();

    const picker = await screen.findByLabelText(messages.Session.language);
    fireEvent.change(picker, { target: { value: "ru" } });
    expect(navigation.replace).toHaveBeenCalledWith("/ru/practice/memorize/signs");
  });

  it("keeps a kiosk language switch inside /station", async () => {
    mockUseMemorize.mockReturnValue({ questions: [question()], loading: false, error: null });
    renderPage(true);

    fireEvent.change(await screen.findByLabelText(messages.Session.language), {
      target: { value: "uz-Cyrl" },
    });
    expect(navigation.replace).toHaveBeenCalledWith("/uz-Cyrl/station/practice/memorize/signs");
  });

  it("leaves the arrow keys to the language picker while it has focus", async () => {
    mockUseMemorize.mockReturnValue({
      questions: [question({ id: "q-1" }), question({ id: "q-2", correct_answer_id: "a-1" })],
      loading: false,
      error: null,
    });
    renderPage();

    const picker = await screen.findByLabelText(messages.Session.language);
    fireEvent.keyDown(picker, { key: "ArrowRight" });
    expect(await screen.findByText("Savol 1 / 2")).toBeInTheDocument();
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
