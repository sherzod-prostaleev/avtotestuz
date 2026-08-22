import { describe, expect, it, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import messages from "../../../../../../../../messages/uz-Latn.json";
import { EnrollFailures, StationHealth } from "./station-health";

function wrap(ui: React.ReactNode) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      {ui}
    </NextIntlClientProvider>,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

const report = {
  created_at: "2026-08-22T10:00:00Z",
  hwid_hash: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
  label: "WIN-CLASS-01",
  agent_version: "1.1.0",
  phase: "blocked",
  code: "hwid_other_org",
  problem: "Bu kompyuter allaqachon BOSHQA avtomaktabga ro'yxatdan o'tgan.",
  detail: "/api/v1/b2b/stations/enroll: conflict",
  os: "windows/386",
  log_tail: "2026/08/22 10:00:00 enrollment refused\n",
};

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

describe("StationHealth", () => {
  it("says nothing has been reported rather than implying health", () => {
    wrap(<StationHealth station={{}} />);
    expect(screen.getByText(messages.AdminB2B.stationNoReport)).toBeInTheDocument();
  });

  it("shows the Uzbek problem the agent wrote, plus its code", () => {
    wrap(
      <StationHealth
        station={{ last_phase: "blocked", last_code: "clock", last_problem: "Soat noto'g'ri." }}
      />,
    );
    expect(screen.getByText(messages.AdminB2B.stationPhase_blocked)).toBeInTheDocument();
    expect(screen.getByText("(clock)")).toBeInTheDocument();
    expect(screen.getByText("Soat noto'g'ri.")).toBeInTheDocument();
  });

  // Two minutes is the backend's tolerance. Inside it the offset is noise;
  // past it every signature this PC makes is rejected, and the operator is
  // shown a message that reads like a revoked station instead.
  it("flags a clock only once it is past what the backend tolerates", () => {
    const { unmount } = wrap(<StationHealth station={{ last_phase: "ready", clock_offset_seconds: 60 }} />);
    expect(screen.queryByText(/daqiqaga farq/)).toBeNull();
    unmount();

    wrap(<StationHealth station={{ last_phase: "blocked", clock_offset_seconds: 900 }} />);
    expect(screen.getByText(/15 daqiqaga farq/)).toBeInTheDocument();
  });
});

describe("EnrollFailures", () => {
  it("lists a PC that never enrolled and reveals its log on demand", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(jsonResponse({ data: [report] }))));
    const user = userEvent.setup();
    wrap(<EnrollFailures orgId="22222222-2222-4222-8222-222222222222" />);

    expect(await screen.findByText(messages.AdminB2B.enrollFailuresTitle)).toBeInTheDocument();
    expect(screen.getByText("WIN-CLASS-01")).toBeInTheDocument();
    expect(screen.getByText(report.problem)).toBeInTheDocument();

    expect(screen.queryByText(/enrollment refused/)).toBeNull();
    await user.click(screen.getByRole("button", { name: messages.AdminB2B.showLog }));
    expect(screen.getByText(/enrollment refused/)).toBeInTheDocument();
  });

  it("stays invisible when every PC connected", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(jsonResponse({ data: [] }))));
    const { container } = wrap(<EnrollFailures orgId="org" />);
    await waitFor(() => expect(container.textContent).toBe(""));
  });

  // The panel is bolted onto a page whose real job is managing licences and
  // seats. A diagnostics endpoint that is unreachable, or that answers with a
  // shape this component did not expect, must never be the reason an operator
  // cannot revoke a station.
  it("survives a rejected request", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.reject(new Error("network"))));
    const { container } = wrap(<EnrollFailures orgId="org" />);
    await waitFor(() => expect(container.textContent).toBe(""));
  });

  it("survives an envelope that is not a list", async () => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(jsonResponse({ data: { org: {} } }))));
    const { container } = wrap(<EnrollFailures orgId="org" />);
    await waitFor(() => expect(container.textContent).toBe(""));
  });
});
