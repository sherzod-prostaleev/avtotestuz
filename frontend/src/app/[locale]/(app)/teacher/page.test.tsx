import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import messages from "../../../../../messages/uz-Latn.json";
import TeacherPage from "./page";

function json(data: unknown): Response {
  return new Response(JSON.stringify({ data }), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => vi.unstubAllGlobals());

describe("Teacher multi-PC guidance", () => {
  it("shows the device-bound login rule and translated B2B labels", async () => {
    const org = {
      id: "33333333-3333-4333-8333-333333333333",
      name: "Yunusobod Avtomaktab",
      status: "active",
      my_role: "teacher",
      member_count: 1,
      active_seats: 10,
    };
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/me/teacher/orgs")) return json([org]);
        if (url.endsWith("/stats")) {
          return json({
            members_total: 1,
            active_seats: 10,
            seats_used: 1,
            seats_remaining: 9,
            avg_readiness_pct: 75,
            sessions_finished_7d: 2,
            sessions_finished_30d: 4,
            active_members_7d: 1,
            pending_invites: 0,
          });
        }
        if (url.endsWith("/invites")) return json([]);
        if (url.endsWith("/stations")) {
          return json([{ id: "station-1", label: "Sinfxona-1", status: "active" }]);
        }
        return json({
          org,
          members: [
            {
              profile_id: "learner-1",
              phone_masked: "+998***2233",
              name: "Ali",
              role: "student",
              readiness_pct: 75,
              has_b2b_vip: true,
            },
          ],
          licenses: [
            {
              id: "license-1",
              seats: 10,
              ends_at: "2027-08-01T00:00:00Z",
              active: true,
              note: "",
            },
          ],
          seats_used: 1,
        });
      }),
    );

    render(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <TeacherPage />
      </NextIntlClientProvider>,
    );

    expect(screen.getByText(messages.Teacher.multiPcTitle)).toBeInTheDocument();
    expect(screen.getByText(messages.Teacher.multiPcHint)).toBeInTheDocument();
    expect(await screen.findByText(/Yunusobod Avtomaktab/)).toBeInTheDocument();
    expect((await screen.findAllByText(/O‘qituvchi/)).length).toBeGreaterThan(0);
    expect(await screen.findByText(/Sinfxona-1 · faol/)).toBeInTheDocument();
    expect(await screen.findByText(/Ali/)).toBeInTheDocument();
    expect(screen.queryByText(/teacher ·/)).not.toBeInTheDocument();
    expect(screen.queryByText(/student/)).not.toBeInTheDocument();
  });
});
