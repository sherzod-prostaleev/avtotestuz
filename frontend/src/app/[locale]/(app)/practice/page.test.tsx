import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, beforeEach } from "vitest";
import messages from "../../../../../messages/uz-Latn.json";
import PracticePage from "./page";
import * as apiClient from "@/lib/api-client";
import { PROTECTED_SEGMENTS, matchesAny } from "@/lib/protected-segments";

/** True once the locale prefix is stripped and every segment is checked
 * against the cookie gate — the same check src/proxy.ts runs on every
 * request from the login-free kiosk browser. */
function isKioskReachable(hrefOrPush: string): boolean {
  const withoutLocale = hrefOrPush.replace(/^\/[a-zA-Z-]+/, "");
  const pathname = withoutLocale.split("?")[0] || "/";
  return !matchesAny(pathname, PROTECTED_SEGMENTS);
}

const { pushMock } = vi.hoisted(() => ({ pushMock: vi.fn() }));

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock, replace: vi.fn() }),
}));

const mockUseSigns = vi.hoisted(() =>
  vi.fn(() => ({ signs: [] as Array<{
    code: string;
    group_code: string;
    name: string;
    image_url: string | null;
    question_count: number;
  }>, loading: false, error: null }))
);

vi.mock("@/hooks/use-signs", () => ({
  useSigns: () => mockUseSigns(),
}));

const CATEGORIES = [
  { code: "driver_duties", name: "Haydovchilarning vazifalari", sort_order: 2, question_count: 88 },
  { code: "general_rules", name: "Umumiy qoidalar", sort_order: 1, question_count: 334 },
];
const VARIANTS = [{ number: 1 }, { number: 2 }, { number: 61 }];
const ALLOWANCE = { unlimited: false, limit: 30, used: 0, remaining: 30 };

function mockEndpoints(overrides: Partial<Record<string, unknown>> = {}) {
  return vi.spyOn(apiClient, "apiGet").mockImplementation(async (path: string) => {
    if (path.startsWith("categories")) return (overrides.categories ?? CATEGORIES) as never;
    if (path.startsWith("variants")) return (overrides.variants ?? VARIANTS) as never;
    if (path.startsWith("me/practice-allowance")) return (overrides.allowance ?? ALLOWANCE) as never;
    if (path.startsWith("me/practice-progress")) return (overrides.progress ?? []) as never;
    if (path.startsWith("me/stats")) return (overrides.stats ?? { due_count: 0 }) as never;
    if (path.startsWith("me/mistakes")) {
      return (overrides.mistakes ?? {
        due_count: 0,
        total_bank_count: 0,
        next_due_at: null,
      }) as never;
    }
    return [] as never;
  });
}

function renderWithIntl() {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <PracticePage />
    </NextIntlClientProvider>
  );
}

describe("PracticePage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    pushMock.mockReset();
    mockUseSigns.mockReturnValue({ signs: [], loading: false, error: null });
  });

  it("sorts categories by sort_order and clicking a category directly starts ordered practice", async () => {
    mockEndpoints();
    renderWithIntl();

    expect(await screen.findByText("Umumiy qoidalar")).toBeInTheDocument();
    expect(apiClient.apiGet).toHaveBeenCalledWith("categories?locale=uz-Latn");

    fireEvent.click(screen.getByText("Umumiy qoidalar").closest("button")!);

    expect(pushMock).toHaveBeenCalledWith(
      "/uz-Latn/session/start?mode=practice&count=1260&category_id=general_rules&ordered=true"
    );
  });

  it("shows the real question count for each category", async () => {
    mockEndpoints();
    renderWithIntl();

    expect(await screen.findByText("334 ta savol")).toBeInTheDocument();
    expect(screen.getByText("88 ta savol")).toBeInTheDocument();
  });

  // The from/to pair is gone: a size is the whole choice, and it draws across
  // every bilet the bank has (61 in this fixture).
  it("starts ticket practice across every bilet when a size is tapped", async () => {
    mockEndpoints();
    renderWithIntl();
    await screen.findByText("Umumiy qoidalar");

    fireEvent.click(screen.getByRole("button", { name: /Biletlar oralig'i/ }));
    fireEvent.click(screen.getByRole("button", { name: "50 ta savol" }));

    expect(pushMock).toHaveBeenCalledWith(
      "/uz-Latn/session/start?mode=practice&count=50&variant_from=1&variant_to=61"
    );
  });

  it("offers the long sizes too, and Hammasi means the whole bank", async () => {
    mockEndpoints();
    renderWithIntl();
    await screen.findByText("Umumiy qoidalar");

    fireEvent.click(screen.getByRole("button", { name: /Biletlar oralig'i/ }));
    expect(screen.getByRole("button", { name: "800 ta savol" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: /^Hammasi$/ }));

    expect(pushMock).toHaveBeenCalledWith(
      "/uz-Latn/session/start?mode=practice&count=1260&variant_from=1&variant_to=61"
    );
  });

  // Without a bilet count the range would be 1..0, which the sessions endpoint
  // rejects — so the sizes stay inert rather than starting a doomed session.
  it("keeps ticket sizes inert until the bilet count is known", async () => {
    mockEndpoints({ variants: [] });
    renderWithIntl();
    await screen.findByText("Umumiy qoidalar");

    fireEvent.click(screen.getByRole("button", { name: /Biletlar oralig'i/ }));
    fireEvent.click(screen.getByRole("button", { name: "20 ta savol" }));

    expect(screen.getByRole("button", { name: "20 ta savol" })).toBeDisabled();
    expect(pushMock).not.toHaveBeenCalled();
  });

  it("starts an image-filtered session from inside practice", async () => {
    mockEndpoints();
    renderWithIntl();
    await screen.findByText("Umumiy qoidalar");

    fireEvent.click(screen.getByRole("button", { name: /Rasm bo'yicha/ }));
    fireEvent.click(screen.getByRole("button", { name: /Rasmsiz savollar/ }));
    fireEvent.click(screen.getByRole("button", { name: "20 ta savol" }));

    expect(pushMock).toHaveBeenCalledWith(
      "/uz-Latn/session/start?mode=practice&count=20&has_image=false"
    );
  });

  it("starts all matching image questions when Hammasi is tapped", async () => {
    mockEndpoints();
    renderWithIntl();
    await screen.findByText("Umumiy qoidalar");

    fireEvent.click(screen.getByRole("button", { name: /Rasm bo'yicha/ }));
    fireEvent.click(screen.getByRole("button", { name: /^Hammasi$/ }));

    expect(pushMock).toHaveBeenCalledWith(
      "/uz-Latn/session/start?mode=practice&count=1260&has_image=true"
    );
  });

  // A learner's level now comes from the practice they do, so the placement
  // test has no entry point left — on the kiosk either.
  it("offers no diagnostic entry point", async () => {
    mockEndpoints();
    renderWithIntl();
    await screen.findByText("Umumiy qoidalar");

    expect(screen.queryByText(/Diagnostika/i)).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /Diagnostika/i })
    ).not.toBeInTheDocument();
  });

  // The size card replaced the one that used to carry the daily budget, so the
  // budget has to survive on its own or the limit is met as a failed start.
  it("still states the remaining daily budget", async () => {
    mockEndpoints();
    renderWithIntl();

    expect(await screen.findByText(/Bugun 30 ta savol qoldi/)).toBeInTheDocument();
  });
});

describe("PracticePage kiosk mode", () => {
  function renderKiosk() {
    return render(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <PracticePage kiosk={true} />
      </NextIntlClientProvider>
    );
  }

  beforeEach(() => {
    vi.restoreAllMocks();
    pushMock.mockReset();
    mockUseSigns.mockReturnValue({ signs: [], loading: false, error: null });
  });

  it("keeps the back link under /station", async () => {
    mockEndpoints();
    renderKiosk();
    const link = (await screen.findByRole("link", { name: /Bosh sahifaga qaytish/ })) as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("/uz-Latn/station");
  });

  it("starts a category practice session on a kiosk-reachable session/start", async () => {
    mockEndpoints();
    renderKiosk();
    await screen.findByText("Umumiy qoidalar");

    fireEvent.click(screen.getByText("Umumiy qoidalar").closest("button")!);

    expect(pushMock).toHaveBeenCalledWith(
      "/uz-Latn/station/session/start?mode=practice&count=1260&category_id=general_rules&ordered=true"
    );
    expect(isKioskReachable(pushMock.mock.calls[0][0])).toBe(true);
  });
});
