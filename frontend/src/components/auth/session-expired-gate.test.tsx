import { render, screen, waitFor } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, apiGet } from "@/lib/api-client";
import { createQueryClient } from "@/lib/query-client";
import { SessionExpiredGate } from "./session-expired-gate";

const replaceMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: replaceMock, push: vi.fn() }),
  usePathname: () => "/uz-Latn/dashboard",
}));

vi.mock("next-intl", () => ({ useLocale: () => "uz-Latn" }));

function renderGate() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <SessionExpiredGate />
      <p>shell</p>
    </QueryClientProvider>,
  );
}

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("SessionExpiredGate", () => {
  beforeEach(() => {
    replaceMock.mockReset();
    vi.unstubAllGlobals();
  });

  it("keeps the shell painted while every call succeeds", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse(200, { data: { ok: true } })),
    );
    renderGate();
    await apiGet("me/streak");
    expect(screen.getByText("shell")).toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });

  // The bug this gate exists for: the session dies AFTER the tab loaded, so
  // /me is never re-fetched and the only 401 handler in the app never fires.
  // Every later call 401s and each caller paints its own "could not load"
  // text, leaving the learner on a dashboard that a reload alone can fix.
  it("sends the learner to login when any later call answers 401", async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/api/auth/logout") return jsonResponse(200, { data: { ok: true } });
      return jsonResponse(401, { error: { code: "unauthorized", message: "session expired" } });
    });
    vi.stubGlobal("fetch", fetchMock);
    renderGate();

    await expect(apiGet("me/sessions?limit=20")).rejects.toBeInstanceOf(ApiError);

    await waitFor(() => {
      expect(replaceMock).toHaveBeenCalledWith("/uz-Latn/login?expired=1");
    });
    // Cookies must be dropped server-side before the redirect: the middleware
    // only checks that `rt` EXISTS, so a lingering cookie bounces /login
    // straight back to /dashboard and the learner ping-pongs forever.
    expect(fetchMock).toHaveBeenCalledWith("/api/auth/logout", { method: "POST" });
  });

  it("redirects once even when a screen fires many parallel calls", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (url === "/api/auth/logout") return jsonResponse(200, { data: { ok: true } });
        return jsonResponse(401, { error: { code: "unauthorized", message: "session expired" } });
      }),
    );
    renderGate();

    await Promise.allSettled([
      apiGet("me"),
      apiGet("me/streak"),
      apiGet("me/stats"),
      apiGet("me/sessions?limit=20"),
    ]);

    await waitFor(() => {
      expect(replaceMock).toHaveBeenCalledTimes(1);
    });
  });

  it("ignores a 401 that arrives after the gate unmounted", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse(401, { error: { code: "unauthorized", message: "no" } })),
    );
    const { unmount } = renderGate();
    unmount();

    await expect(apiGet("me")).rejects.toBeInstanceOf(ApiError);
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it("leaves other failures alone", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse(502, { error: { code: "network_error", message: "down" } })),
    );
    renderGate();

    await expect(apiGet("me/stats")).rejects.toBeInstanceOf(ApiError);
    expect(replaceMock).not.toHaveBeenCalled();
  });
});
