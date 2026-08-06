import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

const apiGet = vi.fn();

vi.mock("@/lib/api-client", () => ({
  apiGet: (...args: unknown[]) => apiGet(...args),
}));

vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => key,
  useLocale: () => "uz-Latn",
}));

import StationPage from "./page";

// station/page.tsx is the fail-closed guard for a login-free classroom
// kiosk: anything that isn't a verified station session must see the
// refusal and nothing else — no practice/tickets entry points reachable.
describe("StationPage", () => {
  afterEach(() => {
    apiGet.mockReset();
  });

  it("renders nothing while the /me check is still in flight", () => {
    apiGet.mockReturnValue(new Promise(() => {})); // never resolves
    const { container } = render(<StationPage />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders the refusal, with no entry points, when GET /me throws", async () => {
    apiGet.mockRejectedValue(new Error("network error"));
    render(<StationPage />);

    await waitFor(() => expect(screen.getByText("notStation")).toBeInTheDocument());
    expect(screen.queryByText("practice")).not.toBeInTheDocument();
    expect(screen.queryByText("exam")).not.toBeInTheDocument();
  });

  it("renders the refusal, with no entry points, for a learner session", async () => {
    apiGet.mockResolvedValue({ kind: "user" });
    render(<StationPage />);

    await waitFor(() => expect(screen.getByText("notStation")).toBeInTheDocument());
    expect(screen.queryByText("practice")).not.toBeInTheDocument();
    expect(screen.queryByText("exam")).not.toBeInTheDocument();
  });

  it("renders the practice and tickets entry points for a station session", async () => {
    apiGet.mockResolvedValue({ kind: "station" });
    render(<StationPage />);

    await waitFor(() => expect(screen.getByText("practice")).toBeInTheDocument());
    expect(screen.getByText("exam")).toBeInTheDocument();
    expect(screen.queryByText("notStation")).not.toBeInTheDocument();
  });

  it("offers the exam simulation", async () => {
    apiGet.mockResolvedValue({ kind: "station", name: "PC-1" });
    render(<StationPage />);

    const link = await screen.findByRole("link", { name: "exam" });
    expect(link).toHaveAttribute("href", "/uz-Latn/station/session/start?mode=exam");
  });
});
