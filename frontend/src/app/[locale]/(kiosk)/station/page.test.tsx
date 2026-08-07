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
// The real GET /me envelope, not a convenient flattening of it: the backend
// answers {"data":{"profile":{...,"kind":"station"},"vip":{...}}} and apiGet
// unwraps only "data". A mock shaped as {kind: "station"} is what let the
// page read me.kind -- always undefined against production -- and ship a
// kiosk that told every classroom PC it was not a classroom PC.
function meResponse(kind: string) {
  return {
    profile: {
      id: "3442327d-982c-4267-a524-ec19cceb251d",
      phone: "st:d32c43dc-3d30-4a9b-af25-9f75d8091edf",
      name: "PC-1",
      region: "",
      district: "",
      birth_date: null,
      locale_pref: "uz-Latn",
      theme_pref: "dark",
      referral_code: "",
      role: "user",
      kind,
      created_at: "2026-08-07T06:26:38Z",
    },
    vip: { active: true, until: "2026-08-12T05:50:42Z" },
  };
}

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
    apiGet.mockResolvedValue(meResponse("user"));
    render(<StationPage />);

    await waitFor(() => expect(screen.getByText("notStation")).toBeInTheDocument());
    expect(screen.queryByText("practice")).not.toBeInTheDocument();
    expect(screen.queryByText("exam")).not.toBeInTheDocument();
  });

  it("renders the practice and tickets entry points for a station session", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    await waitFor(() => expect(screen.getByText("practice")).toBeInTheDocument());
    expect(screen.getByText("exam")).toBeInTheDocument();
    expect(screen.queryByText("notStation")).not.toBeInTheDocument();
  });

  it("offers the exam simulation", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    const link = await screen.findByRole("link", { name: "exam" });
    expect(link).toHaveAttribute("href", "/uz-Latn/station/session/start?mode=exam");
  });

  it("offers road signs", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    const link = await screen.findByRole("link", { name: "signs" });
    expect(link).toHaveAttribute("href", "/uz-Latn/station/signs");
  });

  it("offers stats", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    const link = await screen.findByRole("link", { name: "stats" });
    expect(link).toHaveAttribute("href", "/uz-Latn/station/stats");
  });

  it("offers saved questions", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    const link = await screen.findByRole("link", { name: "saved" });
    expect(link).toHaveAttribute("href", "/uz-Latn/station/saved");
  });

  it("offers the leaderboard", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    const link = await screen.findByRole("link", { name: "leaderboard" });
    expect(link).toHaveAttribute("href", "/uz-Latn/station/leaderboard");
  });
});
