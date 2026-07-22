import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import SavedPage from "./page";
import * as apiClient from "@/lib/api-client";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <SavedPage />
    </NextIntlClientProvider>
  );
}

const questionId = "11111111-1111-4111-8111-111111111111";
const savedDTO = [{ question_id: questionId, created_at: "2026-07-22T12:00:00Z" }];
const questionDTO = {
  id: questionId,
  category_code: "priority_intersections",
  text: "Ushbu chorrahada qaysi transport vositasi birinchi o'tadi?",
  image_url: "https://media.example.test/question-1.webp",
  answers: [{ id: "secret-answer", position: 1, text: "Maxfiy to'g'ri javob", image_url: null }],
  correct_answer_id: "secret-answer",
};

describe("SavedPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads the real saved DTO then localized question details without correctness", async () => {
    vi.spyOn(apiClient, "apiGet")
      .mockResolvedValueOnce(savedDTO)
      .mockResolvedValueOnce(questionDTO);

    renderWithIntl();

    expect(await screen.findByText(questionDTO.text)).toBeInTheDocument();
    expect(apiClient.apiGet).toHaveBeenNthCalledWith(1, "me/saved");
    expect(apiClient.apiGet).toHaveBeenNthCalledWith(
      2,
      `questions/${questionId}?locale=uz-Latn`
    );
    expect(screen.getByText("priority_intersections")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "1-saqlangan savol rasmi" })).toHaveAttribute(
      "src",
      questionDTO.image_url
    );
    expect(screen.queryByText("Maxfiy to'g'ri javob")).not.toBeInTheDocument();
    expect(screen.queryByText("secret-answer")).not.toBeInTheDocument();
  });

  it("renders the honest empty state without requesting question details", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValueOnce([]);

    renderWithIntl();

    expect(await screen.findByText("Hali saqlangan savollar yo'q")).toBeInTheDocument();
    expect(apiClient.apiGet).toHaveBeenCalledTimes(1);
  });

  it("shows a load error and retries the complete request safely", async () => {
    vi.spyOn(apiClient, "apiGet")
      .mockRejectedValueOnce(new Error("network unavailable"))
      .mockResolvedValueOnce([]);

    renderWithIntl();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Saqlangan savollarni yuklab bo'lmadi."
    );
    fireEvent.click(screen.getByRole("button", { name: "Qayta urinish" }));

    expect(await screen.findByText("Hali saqlangan savollar yo'q")).toBeInTheDocument();
    expect(apiClient.apiGet).toHaveBeenCalledTimes(2);
  });

  it("keeps an item after a failed delete and removes it only after a successful retry", async () => {
    vi.spyOn(apiClient, "apiGet")
      .mockResolvedValueOnce(savedDTO)
      .mockResolvedValueOnce(questionDTO);
    vi.spyOn(apiClient, "apiDelete")
      .mockRejectedValueOnce(new Error("delete failed"))
      .mockResolvedValueOnce({ ok: true });

    renderWithIntl();
    expect(await screen.findByText(questionDTO.text)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Saqlanganlardan olib tashlash" }));
    expect(await screen.findByText("Savolni olib tashlab bo'lmadi.")).toBeInTheDocument();
    expect(screen.getByText(questionDTO.text)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Olib tashlashni qayta urinish" }));

    await waitFor(() => expect(screen.queryByText(questionDTO.text)).not.toBeInTheDocument());
    expect(screen.getByText("Hali saqlangan savollar yo'q")).toBeInTheDocument();
    expect(apiClient.apiDelete).toHaveBeenCalledTimes(2);
    expect(apiClient.apiDelete).toHaveBeenNthCalledWith(1, `me/saved/${questionId}`);
    expect(apiClient.apiDelete).toHaveBeenNthCalledWith(2, `me/saved/${questionId}`);
  });
});
