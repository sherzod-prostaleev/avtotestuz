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

  // A classroom PC boots faster than its network: the agent autostarts, its
  // proxy fails closed with 503 station_offline, and it retries in the
  // background. The page used to ask once, catch that, and show the refusal
  // for the rest of the day on a perfectly good station.
  it("waits and retries when GET /me throws, instead of refusing", async () => {
    apiGet.mockRejectedValue(new Error("network error"));
    render(<StationPage />);

    await waitFor(() => expect(screen.getByText("Station.connecting")).toBeInTheDocument());
    expect(screen.queryByText("Station.notStation")).not.toBeInTheDocument();
    expect(screen.queryByText("Station.practice")).not.toBeInTheDocument();

    // And it must actually ask again, not merely display a nicer message.
    await waitFor(() => expect(apiGet.mock.calls.length).toBeGreaterThan(1), { timeout: 4000 });
  });

  it("recovers on its own once the agent has a token", async () => {
    apiGet
      .mockRejectedValueOnce(new Error("station token unavailable"))
      .mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    await waitFor(() => expect(screen.getByText("Station.connecting")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText("Station.practice")).toBeInTheDocument(), {
      timeout: 4000,
    });
    expect(screen.queryByText("Station.connecting")).not.toBeInTheDocument();
    expect(screen.queryByText("Station.notStation")).not.toBeInTheDocument();
  });

  // A real answer that says "not a station" is final -- retrying it would spin
  // forever on a machine that is genuinely not a classroom PC.
  it("refuses a learner session without retrying", async () => {
    apiGet.mockResolvedValue(meResponse("user"));
    render(<StationPage />);

    await waitFor(() => expect(screen.getByText("Station.notStation")).toBeInTheDocument());
    expect(screen.queryByText("Station.practice")).not.toBeInTheDocument();
    expect(screen.queryByText("Station.exam")).not.toBeInTheDocument();

    const callsAfterRefusal = apiGet.mock.calls.length;
    await new Promise((r) => setTimeout(r, 1500));
    expect(apiGet.mock.calls.length).toBe(callsAfterRefusal);
  });

  it("renders the practice and tickets entry points for a station session", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    await waitFor(() => expect(screen.getByText("Station.practice")).toBeInTheDocument());
    expect(screen.getByText("Station.exam")).toBeInTheDocument();
    expect(screen.queryByText("Station.notStation")).not.toBeInTheDocument();
  });

  it("offers the exam simulation", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    const link = await screen.findByRole("link", { name: /Station\.exam/ });
    expect(link).toHaveAttribute("href", "/uz-Latn/station/session/start?mode=exam");
  });

  it("offers road signs", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    const link = await screen.findByRole("link", { name: /Station\.signs/ });
    expect(link).toHaveAttribute("href", "/uz-Latn/station/signs");
  });

  it("offers stats", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    const link = await screen.findByRole("link", { name: /Station\.stats/ });
    expect(link).toHaveAttribute("href", "/uz-Latn/station/stats");
  });

  it("offers saved questions", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    const link = await screen.findByRole("link", { name: /Station\.saved/ });
    expect(link).toHaveAttribute("href", "/uz-Latn/station/saved");
  });

  it("offers the leaderboard", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    const link = await screen.findByRole("link", { name: /Station\.leaderboard/ });
    expect(link).toHaveAttribute("href", "/uz-Latn/station/leaderboard");
  });

  // A station is excluded from the rankings on purpose, so the card must say
  // the board is read-only here rather than reuse Leaderboard.subtitle, which
  // invites the reader to compete for points they can never earn.
  it("says the leaderboard is read-only for this PC", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    expect(await screen.findByText("Station.leaderboardNote")).toBeInTheDocument();
    expect(screen.queryByText("Leaderboard.subtitle")).not.toBeInTheDocument();
  });

  // "Next student" reset nothing a shared PC actually keeps -- it only
  // remounted the screen the student was already looking at -- so it was one
  // more thing on a classroom wall to explain.
  it("offers no next-student button", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    await screen.findByText("Station.title");
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByText(/newStudent/)).not.toBeInTheDocument();
  });

  // The kiosk names the PC so a teacher can tell which machine they are
  // standing at without opening the admin panel.
  it("shows the station's own name", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    expect(await screen.findByText("PC-1")).toBeInTheDocument();
  });

  // Every excluded surface stays excluded after the redesign.
  it("offers no route into mistakes, arena, premium, checkout, profile or dashboard", async () => {
    apiGet.mockResolvedValue(meResponse("station"));
    render(<StationPage />);

    await screen.findByText("Station.title");
    for (const href of screen.getAllByRole("link").map((a) => a.getAttribute("href") ?? "")) {
      expect(href).toMatch(/^\/uz-Latn\/station(\/|$|\?)/);
    }
  });
});
