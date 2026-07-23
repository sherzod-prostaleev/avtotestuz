import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import PracticePage from "./page";
import * as apiClient from "@/lib/api-client";

const { pushMock } = vi.hoisted(() => ({ pushMock: vi.fn() }));

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock, replace: vi.fn() }),
}));

vi.mock("@/hooks/use-signs", () => ({
  useSigns: () => ({ signs: [], loading: false, error: null }),
}));

const CATEGORIES = [
  { code: "stopping_parking", name: "To'xtash va to'xtab turish", sort_order: 2, question_count: 88 },
  { code: "priority_intersections", name: "Chorrahalar", sort_order: 1, question_count: 334 },
];
const VARIANTS = [{ number: 1 }, { number: 2 }, { number: 61 }];
const ALLOWANCE = { unlimited: false, limit: 30, used: 0, remaining: 30 };

function mockEndpoints(overrides: Partial<Record<string, unknown>> = {}) {
  return vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
    if (path.startsWith("categories")) return (overrides.categories ?? CATEGORIES) as never;
    if (path.startsWith("variants")) return (overrides.variants ?? VARIANTS) as never;
    if (path.startsWith("me/practice-allowance")) return (overrides.allowance ?? ALLOWANCE) as never;
    return [] as never;
  });
}

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <PracticePage />
    </NextIntlClientProvider>
  );
}

describe("PracticePage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    pushMock.mockReset();
  });

  it("sorts categories by sort_order and starts practice with the category code", async () => {
    mockEndpoints();
    renderWithIntl();

    expect(await screen.findByText("Chorrahalar")).toBeInTheDocument();
    expect(apiClient.apiGet).toHaveBeenCalledWith("categories?locale=uz-Latn");

    fireEvent.click(screen.getByRole("button", { name: "20 ta savol" }));
    fireEvent.click(screen.getByRole("button", { name: "Mashqni boshlash" }));

    expect(pushMock).toHaveBeenCalledWith(
      "/uz-Latn/session/start?mode=practice&count=20&category_id=priority_intersections"
    );
  });

  // Category sizes differ by an order of magnitude, so the picker states the
  // real count instead of implying the topics are interchangeable.
  it("shows the real question count for each category", async () => {
    mockEndpoints();
    renderWithIntl();

    expect(await screen.findByText("334 ta savol")).toBeInTheDocument();
    expect(screen.getByText("88 ta savol")).toBeInTheDocument();
  });

  it("starts a bilet-span session from the range inputs", async () => {
    mockEndpoints();
    renderWithIntl();
    await screen.findByText("Chorrahalar");

    fireEvent.click(screen.getByRole("button", { name: /Biletlar oralig'i/ }));
    fireEvent.change(screen.getByLabelText("Boshlanishi"), { target: { value: "3" } });
    fireEvent.change(screen.getByLabelText("Tugashi"), { target: { value: "12" } });
    fireEvent.click(screen.getByRole("button", { name: "Mashqni boshlash" }));

    expect(pushMock).toHaveBeenCalledWith(
      "/uz-Latn/session/start?mode=practice&count=20&variant_from=3&variant_to=12"
    );
  });

  it("refuses to start an inverted bilet span", async () => {
    mockEndpoints();
    renderWithIntl();
    await screen.findByText("Chorrahalar");

    fireEvent.click(screen.getByRole("button", { name: /Biletlar oralig'i/ }));
    fireEvent.change(screen.getByLabelText("Boshlanishi"), { target: { value: "20" } });
    fireEvent.change(screen.getByLabelText("Tugashi"), { target: { value: "5" } });

    expect(screen.getByRole("button", { name: "Mashqni boshlash" })).toBeDisabled();
  });

  it("starts an image-filtered session from inside practice", async () => {
    mockEndpoints();
    renderWithIntl();
    await screen.findByText("Chorrahalar");

    fireEvent.click(screen.getByRole("button", { name: /Rasm bo'yicha/ }));
    fireEvent.click(screen.getByRole("button", { name: /Rasmsiz savollar/ }));
    fireEvent.click(screen.getByRole("button", { name: "Mashqni boshlash" }));

    expect(pushMock).toHaveBeenCalledWith(
      "/uz-Latn/session/start?mode=practice&count=20&has_image=false"
    );
  });

  it("accepts a custom question count", async () => {
    mockEndpoints();
    renderWithIntl();
    await screen.findByText("Chorrahalar");

    fireEvent.change(screen.getByLabelText(/O'zim kiritaman/), { target: { value: "40" } });
    fireEvent.click(screen.getByRole("button", { name: "Mashqni boshlash" }));

    expect(pushMock).toHaveBeenCalledWith(
      "/uz-Latn/session/start?mode=practice&count=40&category_id=priority_intersections"
    );
  });

  // The server clamps an oversized request to the remaining budget. Saying so
  // up front is the difference between a visible limit and a broken-looking app.
  it("states the remaining daily budget and warns when a size will be clamped", async () => {
    mockEndpoints({ allowance: { unlimited: false, limit: 30, used: 25, remaining: 5 } });
    renderWithIntl();

    expect(await screen.findByText("Bugun 5 ta savol qoldi")).toBeInTheDocument();
    expect(screen.getByText("Bugun 5 tasi beriladi")).toBeInTheDocument();
  });

  it("blocks starting and offers an upgrade once the budget is spent", async () => {
    mockEndpoints({ allowance: { unlimited: false, limit: 30, used: 30, remaining: 0 } });
    renderWithIntl();

    expect(await screen.findByText("Bugungi bepul limit tugadi")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Premium olish/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Mashqni boshlash" })).toBeDisabled();
  });

  it("disables the sign source while the sign catalogue is empty", async () => {
    mockEndpoints();
    renderWithIntl();
    await screen.findByText("Chorrahalar");

    expect(screen.getByRole("button", { name: /Belgilar bazasi hali to'ldirilmagan/ })).toBeDisabled();
  });

  it("shows an honest error with retry and never invents fallback categories", async () => {
    // Fail the categories call specifically: the page loads it alongside
    // variants and the allowance, so a "first call fails" mock would attach to
    // whichever request happened to be scheduled first.
    let categoryAttempts = 0;
    vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
      if (path.startsWith("categories")) {
        categoryAttempts += 1;
        if (categoryAttempts === 1) throw new Error("network unavailable");
        return CATEGORIES as never;
      }
      if (path.startsWith("variants")) return VARIANTS as never;
      if (path.startsWith("me/practice-allowance")) return ALLOWANCE as never;
      return [] as never;
    });

    renderWithIntl();

    expect(await screen.findByRole("alert")).toHaveTextContent("Kategoriyalarni yuklab bo'lmadi.");
    expect(screen.queryByText("Chorrahalar")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Qayta urinish" }));

    expect(await screen.findByText("Chorrahalar")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole("button", { name: "Mashqni boshlash" })).toBeEnabled());
  });
});
