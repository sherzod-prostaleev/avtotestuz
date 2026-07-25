import { describe, expect, it, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MaintenanceBanner } from "./maintenance-banner";
import * as apiClient from "@/lib/api-client";

vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => key,
}));

describe("MaintenanceBanner", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows when maintenance_mode is true", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({ maintenance_mode: true });
    render(<MaintenanceBanner />);
    expect(await screen.findByText("message")).toBeInTheDocument();
  });

  it("hides when maintenance_mode is false", async () => {
    vi.spyOn(apiClient, "apiGet").mockResolvedValue({ maintenance_mode: false });
    render(<MaintenanceBanner />);
    await waitFor(() => {
      expect(apiClient.apiGet).toHaveBeenCalledWith("flags");
    });
    expect(screen.queryByText("message")).not.toBeInTheDocument();
  });
});
