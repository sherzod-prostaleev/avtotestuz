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

  it("loads the localized real DTO, sorts by sort_order, and starts with the category code", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue([
      { code: "stopping_parking", name: "To'xtash va to'xtab turish", sort_order: 2 },
      { code: "priority_intersections", name: "Chorrahalar", sort_order: 1 },
    ]);

    renderWithIntl();

    expect(screen.getByText("Mashq rejimi")).toBeInTheDocument();
    expect(screen.getByText("Kategoriya bo'yicha")).toBeInTheDocument();
    expect(await screen.findByText("Chorrahalar")).toBeInTheDocument();
    expect(apiClient.apiGet).toHaveBeenCalledWith("categories?locale=uz-Latn");

    fireEvent.click(screen.getByRole("button", { name: "20 ta savol" }));
    fireEvent.click(screen.getByRole("button", { name: "Mashqni boshlash" }));

    expect(pushMock).toHaveBeenCalledWith(
      "/uz-Latn/session/start?mode=practice&category_id=priority_intersections&count=20"
    );
  });

  it("shows an honest error with retry and never invents fallback categories", async () => {
    vi.spyOn(apiClient, "apiGet")
      .mockRejectedValueOnce(new Error("network unavailable"))
      .mockResolvedValueOnce([
        { code: "road_signs_markings", name: "Yo'l belgilari", sort_order: 1 },
      ]);

    renderWithIntl();

    expect(await screen.findByRole("alert")).toHaveTextContent("Kategoriyalarni yuklab bo'lmadi.");
    expect(screen.queryByText("Chorrahalar va yo'l ustunligi")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Mashqni boshlash" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Qayta urinish" }));

    expect(await screen.findByText("Yo'l belgilari")).toBeInTheDocument();
    await waitFor(() => expect(apiClient.apiGet).toHaveBeenCalledTimes(2));
    expect(screen.getByRole("button", { name: "Mashqni boshlash" })).toBeEnabled();
  });
});
