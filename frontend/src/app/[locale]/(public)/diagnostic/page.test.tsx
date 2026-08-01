import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import { DEMO_PROGRESS_STORAGE_KEY } from "@/lib/demo-progress-storage";
import PublicDiagnosticPage from "./page";

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a href={String(href)} {...props}>{children}</a>
  ),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/uz-Latn/diagnostic",
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
}));

vi.mock("@/components/theme-toggle", () => ({ ThemeToggle: () => null }));

const question = {
  id: "question-1",
  text: "Diagnostika savoli",
  image_url: null,
  answers: [
    { id: "answer-1", text: "Birinchi javob" },
    { id: "answer-2", text: "Ikkinchi javob" },
  ],
};

function renderPage() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <PublicDiagnosticPage />
    </NextIntlClientProvider>,
  );
}

beforeEach(() => {
  window.localStorage.clear();
  vi.stubGlobal(
    "fetch",
    vi.fn().mockImplementation((input: string) => {
      if (input.includes("/demo/diagnostic")) {
        return Promise.resolve(
          new Response(
            JSON.stringify({ data: { questions: [question], total_seconds: 720 } }),
            { status: 200, headers: { "Content-Type": "application/json" } },
          ),
        );
      }
      if (input.includes("/demo/answer")) {
        return Promise.resolve(
          new Response(
            JSON.stringify({ data: { correct: false, correct_answer_id: "answer-2" } }),
            { status: 200, headers: { "Content-Type": "application/json" } },
          ),
        );
      }
      return Promise.reject(new Error("unexpected request"));
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("PublicDiagnosticPage", () => {
  it("runs without authentication, grades an answer, and keeps guest progress", async () => {
    renderPage();

    expect(await screen.findByText("Diagnostika savoli")).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith(
      "/api/proxy/demo/diagnostic?locale=uz-Latn",
      expect.objectContaining({ cache: "no-store" }),
    );

    fireEvent.click(screen.getByText("Birinchi javob"));
    expect(await screen.findByText("Noto'g'ri javob")).toBeInTheDocument();
    await waitFor(() => {
      expect(window.localStorage.getItem(DEMO_PROGRESS_STORAGE_KEY)).toContain("question-1");
    });

    fireEvent.click(screen.getByRole("button", { name: /Natijani ko'rish/ }));
    expect(await screen.findByText("Diagnostika yakunlandi")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Bepul ro'yxatdan o'tish/ })).toHaveAttribute(
      "href",
      "/uz-Latn/register",
    );
  });
});
