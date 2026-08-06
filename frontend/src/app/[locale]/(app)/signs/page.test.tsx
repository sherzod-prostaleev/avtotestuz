import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import * as useSignsModule from "@/hooks/use-signs";
import SignsPage from "./page";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

const testMessages = {
  ...messages,
  Signs: {
    ...messages.Signs,
  },
};

const sign = {
  code: "3.27",
  group_code: "prohibiting",
  name: "To'xtash taqiqlangan",
  image_url: "/signs/3.27.png",
  question_count: 4,
};

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={testMessages}>
      <SignsPage />
    </NextIntlClientProvider>
  );
}

function mockSigns(overrides: Partial<ReturnType<typeof useSignsModule.useSigns>> = {}) {
  const result = {
    signs: [sign],
    loading: false,
    error: null,
    refetch: vi.fn(),
    ...overrides,
  };
  vi.spyOn(useSignsModule, "useSigns").mockReturnValue(result);
  return result;
}

describe("SignsPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(useSignsModule, "useSignDetail").mockReturnValue({
      sign: null,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
  });

  it("renders the server-provided sign summary with question count", () => {
    mockSigns();

    renderWithIntl();

    expect(screen.getByRole("heading", { name: "Yo'l belgilari katalogi" })).toBeInTheDocument();
    expect(screen.getByText("3.27")).toBeInTheDocument();
    expect(screen.getByText("To'xtash taqiqlangan")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(useSignsModule.useSigns).toHaveBeenCalledWith("uz-Latn", "all", "");
  });

  it("links signs with questions straight into practice", () => {
    mockSigns();

    renderWithIntl();

    const link = screen.getByRole("link", { name: /To'xtash taqiqlangan/ });
    expect(link).toHaveAttribute(
      "href",
      "/uz-Latn/session/start?mode=practice&sign_id=3.27&count=4"
    );
  });

  it("opens the modal for catalog-only signs without linked questions", () => {
    mockSigns({ signs: [{ ...sign, question_count: 0 }] });
    vi.mocked(useSignsModule.useSignDetail).mockReturnValue({
      sign: {
        code: "3.27",
        group_code: "prohibiting",
        name: "To'xtash taqiqlangan",
        description: "Transport vositalarining to'xtashi taqiqlanadi.",
        image_url: "/signs/3.27.png",
        question_ids: [],
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    renderWithIntl();
    fireEvent.click(screen.getByRole("button", { name: /To'xtash taqiqlangan/ }));

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText("Transport vositalarining to'xtashi taqiqlanadi.")).toBeInTheDocument();
    expect(useSignsModule.useSignDetail).toHaveBeenLastCalledWith("3.27", "uz-Latn");
  });

  it("shows an API-unavailable state and lets the user retry", () => {
    const refetch = vi.fn();
    mockSigns({ signs: [], error: "unavailable", refetch });

    renderWithIntl();

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Yo'l belgilarini serverdan yuklab bo'lmadi."
    );
    fireEvent.click(screen.getByRole("button", { name: "Qayta urinish" }));
    expect(refetch).toHaveBeenCalledTimes(1);
    expect(screen.queryByText(testMessages.Signs.emptyCatalog)).not.toBeInTheDocument();
  });

  it("identifies an empty unfiltered response as an empty server catalog", () => {
    mockSigns({ signs: [] });

    renderWithIntl();

    expect(screen.getByText(testMessages.Signs.emptyCatalog)).toBeInTheDocument();
    expect(screen.queryByText(testMessages.Signs.noResults)).not.toBeInTheDocument();
  });

  it("distinguishes no search matches from an empty catalog", () => {
    mockSigns({ signs: [] });

    renderWithIntl();
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "3.27" } });

    expect(screen.getByText(testMessages.Signs.noResults)).toBeInTheDocument();
    expect(screen.queryByText(testMessages.Signs.emptyCatalog)).not.toBeInTheDocument();
  });

  it("does not invent a legal description when the verified detail is empty", () => {
    mockSigns({ signs: [{ ...sign, question_count: 0 }] });
    vi.mocked(useSignsModule.useSignDetail).mockReturnValue({
      sign: {
        code: "3.27",
        group_code: "prohibiting",
        name: "To'xtash taqiqlangan",
        description: "",
        image_url: null,
        question_ids: [],
      },
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    renderWithIntl();
    fireEvent.click(screen.getByRole("button", { name: /To'xtash taqiqlangan/ }));

    expect(screen.getByText(testMessages.Signs.descriptionUnavailable)).toBeInTheDocument();
    expect(
      screen.queryByText(/O'zbekiston Respublikasi YHQ standartiga mos keladi/)
    ).not.toBeInTheDocument();
  });

  it("shows an honest media state when the API has no image", () => {
    mockSigns({ signs: [{ ...sign, image_url: null, question_count: 0 }] });

    renderWithIntl();

    expect(screen.getByText(testMessages.Signs.imageUnavailable)).toBeInTheDocument();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });
});

// Walks every navigation this page can perform for a cookie-less classroom
// kiosk browser (frontend/src/app/[locale]/(kiosk)/station/signs/page.tsx
// reuses this component with kiosk=true). Both targets below are built at
// runtime (backHref/practiceHref are variables, not string literals), so
// the lexical scan in (kiosk)/kiosk-path.test.tsx cannot see them — this is
// what makes them visible, per the task brief.
describe("SignsPage kiosk mode", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(useSignsModule, "useSignDetail").mockReturnValue({
      sign: null,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });
  });

  function renderKiosk() {
    return render(
      <NextIntlClientProvider locale="uz-Latn" messages={testMessages}>
        <SignsPage kiosk />
      </NextIntlClientProvider>
    );
  }

  it("keeps the back link under /station/practice instead of the gated /practice", () => {
    mockSigns();
    renderKiosk();

    const backLink = screen.getByRole("link", { name: /Mashq rejimiga qaytish/ });
    expect(backLink).toHaveAttribute("href", "/uz-Latn/station/practice");
  });

  it("starts a sign practice session under /station/session/start", () => {
    mockSigns();
    renderKiosk();

    const link = screen.getByRole("link", { name: /To'xtash taqiqlangan/ });
    expect(link).toHaveAttribute(
      "href",
      "/uz-Latn/station/session/start?mode=practice&sign_id=3.27&count=4"
    );
  });
});
