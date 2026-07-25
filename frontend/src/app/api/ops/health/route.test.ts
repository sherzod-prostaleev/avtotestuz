import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/backend", () => ({
  backendRootFetch: vi.fn(),
}));

import { backendRootFetch } from "@/lib/backend";
import { GET } from "./route";

const mockedFetch = vi.mocked(backendRootFetch);

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("GET /api/ops/health", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("returns ok when live and ready succeed", async () => {
    mockedFetch.mockImplementation(async (path: string) => {
      if (path === "/healthz") {
        return jsonResponse(200, { data: { status: "ok" } });
      }
      if (path === "/metrics") {
        return jsonResponse(200, {
          data: {
            uptime_seconds: 12,
            requests_total: 3,
            requests_by_status_class: { "2xx": 2, "3xx": 0, "4xx": 1, "5xx": 0, other: 0 },
          },
        });
      }
      return jsonResponse(200, {
        data: { status: "ok", checks: { postgres: "ok", redis: "ok" } },
      });
    });

    const res = await GET();
    expect(res.status).toBe(200);
    const body = await res.json();
    expect(body.data.status).toBe("ok");
    expect(body.data.live.ok).toBe(true);
    expect(body.data.ready.ok).toBe(true);
    expect(body.data.metrics.ok).toBe(true);
    expect(body.data.metrics.data.data.requests_total).toBe(3);
    expect(body.data.checked_at).toMatch(/^\d{4}-/);
  });

  it("returns degraded 503 when readiness fails", async () => {
    mockedFetch.mockImplementation(async (path: string) => {
      if (path === "/healthz") {
        return jsonResponse(200, { data: { status: "ok" } });
      }
      if (path === "/metrics") {
        return jsonResponse(200, { data: { uptime_seconds: 1, requests_total: 0 } });
      }
      return jsonResponse(503, {
        data: { status: "not_ready", checks: { postgres: "fail", redis: "ok" } },
      });
    });

    const res = await GET();
    expect(res.status).toBe(503);
    const body = await res.json();
    expect(body.data.status).toBe("degraded");
    expect(body.data.ready.ok).toBe(false);
  });
});
