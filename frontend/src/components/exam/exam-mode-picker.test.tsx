import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import messages from "../../../messages/uz-Latn.json";
import { ExamModePicker } from "./exam-mode-picker";
import { useMeQuery } from "@/hooks/use-me";
import { PROTECTED_SEGMENTS, matchesAny } from "@/lib/protected-segments";

const navigation = vi.hoisted(() => ({ push: vi.fn() }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: navigation.push }),
}));

vi.mock("@/hooks/use-me", () => ({ useMeQuery: vi.fn() }));

/** Same check src/proxy.ts runs on every request from a login-free kiosk browser. */
function isKioskReachable(target: string): boolean {
  const withoutLocale = target.replace(/^\/[a-zA-Z-]+/, "");
  const pathname = withoutLocale.split("?")[0] || "/";
  return !matchesAny(pathname, PROTECTED_SEGMENTS);
}

function mockVip(active: boolean) {
  vi.mocked(useMeQuery).mockReturnValue({
    data: { vip: { active } },
  } as ReturnType<typeof useMeQuery>);
}

function renderPicker(kiosk = false) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ExamModePicker kiosk={kiosk} />
    </NextIntlClientProvider>
  );
}

describe("ExamModePicker", () => {
  beforeEach(() => {
    navigation.push.mockReset();
    vi.mocked(useMeQuery).mockReset();
    mockVip(true);
  });

  it("offers both official exam varieties", () => {
    renderPicker();
    expect(screen.getByText(messages.ExamPicker.standardTitle)).toBeInTheDocument();
    expect(screen.getByText(messages.ExamPicker.restoreTitle)).toBeInTheDocument();
  });

  it("states each variety's real rules", () => {
    renderPicker();
    expect(screen.getByText("20 savol · 25 daqiqa")).toBeInTheDocument();
    expect(screen.getByText("50 savol · 50 daqiqa")).toBeInTheDocument();
    expect(screen.getByText("2 xatogacha ruxsat")).toBeInTheDocument();
    expect(screen.getByText("4 xatogacha ruxsat")).toBeInTheDocument();
    expect(screen.getByText("3-xatoda to'xtaydi")).toBeInTheDocument();
    expect(screen.getByText("5-xatoda to'xtaydi")).toBeInTheDocument();
  });

  it("starts a 20-question exam from the standard card", () => {
    renderPicker();
    fireEvent.click(screen.getByTestId("exam-mode-standard"));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/session/start?mode=exam&count=20");
  });

  it("starts a 50-question exam from the restore card", () => {
    renderPicker();
    fireEvent.click(screen.getByTestId("exam-mode-restore"));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/session/start?mode=exam&count=50");
  });

  // A classroom PC has a keyboard and often no usable mouse.
  it("starts an exam from the number keys", () => {
    renderPicker();
    fireEvent.keyDown(window, { key: "2" });
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/session/start?mode=exam&count=50");
  });

  it("only draws the counts the backend actually whitelists", () => {
    renderPicker();
    fireEvent.click(screen.getByTestId("exam-mode-standard"));
    fireEvent.click(screen.getByTestId("exam-mode-restore"));
    for (const [target] of navigation.push.mock.calls) {
      expect(target).toMatch(/[?&]count=(20|50)$/);
    }
  });
});

describe("ExamModePicker on the kiosk", () => {
  beforeEach(() => {
    navigation.push.mockReset();
    vi.mocked(useMeQuery).mockReset();
    // A station profile has no /me VIP payload of its own to lean on.
    mockVip(false);
  });

  it("keeps every destination inside the station namespace", () => {
    renderPicker(true);
    fireEvent.click(screen.getByTestId("exam-mode-restore"));

    const [target] = navigation.push.mock.calls[0];
    expect(target).toBe("/uz-Latn/station/session/start?mode=exam&count=50");
    expect(isKioskReachable(target)).toBe(true);
  });

  it("never offers the paywall to a walk-up student", () => {
    renderPicker(true);
    expect(screen.queryByText(messages.ExamPicker.vipLocked)).not.toBeInTheDocument();
  });
});

describe("ExamModePicker without a subscription", () => {
  beforeEach(() => {
    navigation.push.mockReset();
    vi.mocked(useMeQuery).mockReset();
    mockVip(false);
  });

  it("sends a learner without VIP to the tariffs instead of into a 402", () => {
    renderPicker();
    fireEvent.click(screen.getByTestId("exam-mode-standard"));
    expect(navigation.push).toHaveBeenCalledWith("/uz-Latn/premium");
  });

  it("says why the card is locked", () => {
    renderPicker();
    expect(screen.getAllByText(messages.ExamPicker.vipLocked)).toHaveLength(2);
  });
});
