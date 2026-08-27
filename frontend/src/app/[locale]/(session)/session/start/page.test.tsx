import { StrictMode } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../../../messages/uz-Latn.json";
import SessionStartPage from "./page";
import { useSessionEngine, type SessionError } from "@/hooks/use-session-engine";
import { PROTECTED_SEGMENTS, matchesAny } from "@/lib/protected-segments";

/** Same check src/proxy.ts runs on every request from a login-free kiosk browser. */
function isKioskReachable(target: string): boolean {
  const withoutLocale = target.replace(/^\/[a-zA-Z-]+/, "");
  const pathname = withoutLocale.split("?")[0] || "/";
  return !matchesAny(pathname, PROTECTED_SEGMENTS);
}

const navigation = vi.hoisted(() => ({
  searchParams: new URLSearchParams(),
  push: vi.fn(),
  replace: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => navigation.searchParams,
  useRouter: () => ({ push: navigation.push, replace: navigation.replace }),
}));

vi.mock("@/hooks/use-session-engine", () => ({
  useSessionEngine: vi.fn(),
}));

type SessionEngine = ReturnType<typeof useSessionEngine>;

function mockSessionEngine(error: SessionError | null = null) {
  const startSession = vi.fn().mockResolvedValue(null);
  vi.mocked(useSessionEngine).mockReturnValue({
    session: null,
    loading: false,
    submitting: false,
    error,
    loadSession: vi.fn(),
    startSession,
    submitAnswer: vi.fn(),
    finishSession: vi.fn(),
  } as SessionEngine);
  return startSession;
}

function renderPage(strict = false) {
  const page = (
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <SessionStartPage />
    </NextIntlClientProvider>
  );

  return render(strict ? <StrictMode>{page}</StrictMode> : page);
}

function renderKioskPage() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <SessionStartPage kiosk />
    </NextIntlClientProvider>
  );
}

describe("SessionStartPage", () => {
  beforeEach(() => {
    navigation.searchParams = new URLSearchParams();
    navigation.push.mockReset();
    navigation.replace.mockReset();
    vi.mocked(useSessionEngine).mockReset();
  });

  it("forwards variant_id and the current locale", async () => {
    navigation.searchParams = new URLSearchParams("mode=variant&variant_id=61");
    const startSession = mockSessionEngine();

    renderPage();

    await waitFor(() =>
      expect(startSession).toHaveBeenCalledWith("variant", {
        variant_id: "61",
        locale: "uz-Latn",
      })
    );
  });

  it("forwards category_id, count, and the current locale", async () => {
    navigation.searchParams = new URLSearchParams(
      "mode=practice&category_id=priority_intersections&count=20"
    );
    const startSession = mockSessionEngine();

    renderPage();

    await waitFor(() =>
      expect(startSession).toHaveBeenCalledWith("practice", {
        category_id: "priority_intersections",
        question_count: 20,
        locale: "uz-Latn",
      })
    );
  });

  it("forwards sign_id, count, and the current locale", async () => {
    navigation.searchParams = new URLSearchParams("mode=practice&sign_id=3.27&count=10");
    const startSession = mockSessionEngine();

    renderPage();

    await waitFor(() =>
      expect(startSession).toHaveBeenCalledWith("practice", {
        sign_id: "3.27",
        question_count: 10,
        locale: "uz-Latn",
      })
    );
  });

  it("forwards variant_from/variant_to, count, and the current locale", async () => {
    navigation.searchParams = new URLSearchParams(
      "mode=practice&variant_from=1&variant_to=60&count=50"
    );
    const startSession = mockSessionEngine();

    renderPage();

    await waitFor(() =>
      expect(startSession).toHaveBeenCalledWith("practice", {
        variant_from: 1,
        variant_to: 60,
        question_count: 50,
        locale: "uz-Latn",
      })
    );
  });

  it("forwards has_image, count, and the current locale", async () => {
    navigation.searchParams = new URLSearchParams(
      "mode=practice&has_image=false&count=20"
    );
    const startSession = mockSessionEngine();

    renderPage();

    await waitFor(() =>
      expect(startSession).toHaveBeenCalledWith("practice", {
        has_image: false,
        question_count: 20,
        locale: "uz-Latn",
      })
    );
  });

  it("starts only once when React StrictMode re-runs the effect", async () => {
    navigation.searchParams = new URLSearchParams("mode=exam");
    const startSession = mockSessionEngine();

    renderPage(true);

    await waitFor(() => expect(startSession).toHaveBeenCalledTimes(1));
  });

  it("renders a localized generic SessionError without leaking raw backend text", async () => {
    mockSessionEngine({ code: "network_error", message: "Backend bilan aloqa yo'q" });

    renderPage();

    expect(screen.getByText(/Tarmoq bilan aloqa uzildi/)).toBeInTheDocument();
    expect(screen.queryByText("Backend bilan aloqa yo'q")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Biletlarga qaytish" })).toBeInTheDocument();
  });

  it("maps vip_required to the premium page with clear guidance", () => {
    mockSessionEngine({ code: "vip_required", message: "active entitlement required" });

    renderPage();

    expect(screen.getByText("Bu sessiyani boshlash uchun VIP obunasi kerak.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "VIP tariflarini ko'rish" }));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/premium");
  });

  it("maps previous_ticket_required back to tickets with unlock guidance", () => {
    mockSessionEngine({
      code: "previous_ticket_required",
      message: "complete the previous ticket first",
    });

    renderPage();

    expect(
      screen.getByText(/Avval oldingi biletda kamida 10 ta to'g'ri/i)
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Biletlarga qaytish" }));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/tickets");
  });

  it("maps daily_limit_reached back to practice with the localized limit message", () => {
    mockSessionEngine({ code: "daily_limit_reached", message: "daily practice limit reached" });

    renderPage();

    expect(
      screen.getByText(
        "Bugungi bepul mashq limitingiz tugadi. Ertaga davom eting yoki VIP obunasini oling!"
      )
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Mavzularga qaytish" }));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/practice");
  });
});

// Walks every exit this screen can take for a cookie-less classroom kiosk
// browser (frontend/src/app/[locale]/(kiosk)/station/session/start/page.tsx
// reuses this component with kiosk=true) and checks each destination against
// the same PROTECTED_SEGMENTS gate src/proxy.ts enforces. This is the exact
// screen the previous fix round left broken: practice/tickets pushed here,
// but the success/error exits were still all under the login-gated /session,
// /tickets, /practice, /premium, and /dashboard routes.
describe("SessionStartPage kiosk mode", () => {
  beforeEach(() => {
    navigation.searchParams = new URLSearchParams();
    navigation.push.mockReset();
    navigation.replace.mockReset();
    vi.mocked(useSessionEngine).mockReset();
  });

  it("replaces to a kiosk-reachable session route on success", async () => {
    navigation.searchParams = new URLSearchParams("mode=variant&variant_id=1");
    vi.mocked(useSessionEngine).mockReturnValue({
      session: null,
      loading: false,
      submitting: false,
      error: null,
      loadSession: vi.fn(),
      startSession: vi.fn().mockResolvedValue({ id: "sess-kiosk-1" }),
      submitAnswer: vi.fn(),
      finishSession: vi.fn(),
    } as SessionEngine);

    renderKioskPage();

    await waitFor(() => expect(navigation.replace).toHaveBeenCalledWith("/uz-Latn/station/session/sess-kiosk-1"));
    expect(isKioskReachable(navigation.replace.mock.calls[0][0])).toBe(true);
  });

  it.each([
    ["vip_required", "active entitlement required", "Sinfxonaga qaytish"],
    ["previous_ticket_required", "complete the previous ticket first", "Biletlarga qaytish"],
    ["daily_limit_reached", "daily practice limit reached", "Mavzularga qaytish"],
    ["mock_not_eligible", "not eligible for grand mock", "Sinfxonaga qaytish"],
    ["nothing_due", "nothing due for review", "Mavzularga qaytish"],
    ["network_error", "backend unreachable", "Biletlarga qaytish"],
  ])("routes the %s error to a kiosk-reachable destination", (code, message, actionLabel) => {
    mockSessionEngine({ code, message } as SessionError);

    renderKioskPage();

    fireEvent.click(screen.getByRole("button", { name: actionLabel }));
    expect(navigation.push).toHaveBeenCalledTimes(1);
    const target = navigation.push.mock.calls[0][0] as string;
    expect(isKioskReachable(target)).toBe(true);
    // Never /premium or any other purchase/dashboard route the kiosk hides.
    expect(target.startsWith("/uz-Latn/station")).toBe(true);
  });
});
