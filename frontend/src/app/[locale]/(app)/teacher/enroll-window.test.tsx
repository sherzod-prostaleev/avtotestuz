import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import EnrollWindowPanel from "./enroll-window-panel";

const apiGet = vi.fn();
const apiPost = vi.fn();
const apiDelete = vi.fn();

vi.mock("@/lib/api-client", () => ({
  apiGet: (...args: unknown[]) => apiGet(...args),
  apiPost: (...args: unknown[]) => apiPost(...args),
  apiDelete: (...args: unknown[]) => apiDelete(...args),
}));

vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => key,
}));

describe("EnrollWindowPanel", () => {
  beforeEach(() => {
    apiGet.mockReset();
    apiPost.mockReset();
    apiDelete.mockReset();
  });

  it("opens a window and shows the code", async () => {
    apiGet.mockResolvedValue(null);
    apiPost.mockResolvedValue({
      id: "code-1",
      code: "AVTO-ABCD-EFGH",
      max_uses: 30,
      used_count: 0,
      expires_at: new Date(Date.now() + 7_200_000).toISOString(),
    });

    render(<EnrollWindowPanel orgId="org-1" />);

    await waitFor(() => expect(screen.getByText("enrollNone")).toBeInTheDocument());

    await userEvent.click(screen.getByRole("button", { name: "enrollOpen" }));

    await waitFor(() => expect(screen.getByText("AVTO-ABCD-EFGH")).toBeInTheDocument());
    expect(apiPost).toHaveBeenCalledWith("me/teacher/orgs/org-1/enroll-window", { ttl_minutes: 120 });
  });

  it("closes an open window", async () => {
    apiGet.mockResolvedValue({
      id: "code-1",
      code: "AVTO-ABCD-EFGH",
      max_uses: 30,
      used_count: 4,
      expires_at: new Date(Date.now() + 7_200_000).toISOString(),
    });
    apiDelete.mockResolvedValue(undefined);

    render(<EnrollWindowPanel orgId="org-1" />);

    await waitFor(() => expect(screen.getByText("AVTO-ABCD-EFGH")).toBeInTheDocument());

    await userEvent.click(screen.getByRole("button", { name: "enrollClose" }));

    await waitFor(() =>
      expect(apiDelete).toHaveBeenCalledWith("me/teacher/orgs/org-1/enroll-window/code-1"),
    );
  });
});
