import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { SupportBanner } from "./support-banner";
import * as apiClient from "@/lib/api-client";

vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => key,
}));

vi.mock("next/link", () => ({
  default: ({ href, children }: { href: string; children: React.ReactNode }) => (
    <a href={href}>{children}</a>
  ),
}));

describe("SupportBanner", () => {
  beforeEach(() => {
    sessionStorage.clear();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders enabled banner and dismisses", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      enabled: true,
      message: "Test banner",
      href: "/uz-Latn/premium",
      updated_at: "2026-07-26T00:00:00Z",
    });
    render(<SupportBanner />);
    expect(await screen.findByText("Test banner")).toBeInTheDocument();
    expect(screen.getByRole("link")).toHaveAttribute("href", "/uz-Latn/premium");
    fireEvent.click(screen.getByLabelText("dismiss"));
    await waitFor(() => {
      expect(screen.queryByText("Test banner")).not.toBeInTheDocument();
    });
  });

  it("stays hidden when disabled", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({
      enabled: false,
      message: "Nope",
    });
    render(<SupportBanner />);
    await waitFor(() => {
      expect(apiClient.apiGet).toHaveBeenCalledWith("site/banner");
    });
    expect(screen.queryByText("Nope")).not.toBeInTheDocument();
  });
});
