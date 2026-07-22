import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import PracticePage from "./page";
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
      <PracticePage />
    </NextIntlClientProvider>
  );
}

describe("PracticePage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("renders practice options and start button", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue([
      { id: "c1", code: "priority_intersections", name: "Chorrahalar" },
    ] as any);

    renderWithIntl();

    expect(screen.getByText("Mashq rejimi")).toBeInTheDocument();
    expect(screen.getByText("Kategoriya bo'yicha")).toBeInTheDocument();
  });
});
