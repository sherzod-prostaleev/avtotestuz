import { render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import MistakesPage from "./page";
import * as apiClient from "@/lib/api-client";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <MistakesPage />
    </NextIntlClientProvider>
  );
}

describe("MistakesPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders due count and bank count", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      due_count: 5,
      total_bank_count: 12,
    } as any);

    renderWithIntl();

    expect(screen.getByText("Xatolar ustida ishlash")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("5")).toBeInTheDocument();
      expect(screen.getByText("12")).toBeInTheDocument();
    });
  });
});
