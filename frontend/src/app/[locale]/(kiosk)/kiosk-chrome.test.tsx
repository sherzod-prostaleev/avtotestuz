import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { KioskChrome } from "./kiosk-chrome";

const pathname = vi.hoisted(() => ({ value: "/uz-Latn/station" }));

vi.mock("next/navigation", () => ({
  usePathname: () => pathname.value,
}));

vi.mock("@/components/locale-switcher", () => ({
  LocaleSwitcher: () => <div data-testid="locale-switcher" />,
}));

vi.mock("@/components/theme-toggle", () => ({
  ThemeToggle: () => <div data-testid="theme-toggle" />,
}));

describe("KioskChrome", () => {
  it("offers language and theme on the kiosk home", () => {
    pathname.value = "/uz-Latn/station";
    render(<KioskChrome />);

    expect(screen.getByTestId("locale-switcher")).toBeInTheDocument();
    expect(screen.getByTestId("theme-toggle")).toBeInTheDocument();
  });

  it.each([
    "/uz-Latn/station/practice",
    "/ru/station/tickets",
    "/uz-Cyrl/station/leaderboard",
    "/uz-Latn/station/session/start",
  ])("offers them in every section, including %s", (path) => {
    pathname.value = path;
    render(<KioskChrome />);

    expect(screen.getByTestId("locale-switcher")).toBeInTheDocument();
    expect(screen.getByTestId("theme-toggle")).toBeInTheDocument();
  });

  // The running exam is full-screen and already carries its own language
  // control in its header row; a second floating bar would sit over the
  // question.
  it.each(["/uz-Latn/station/session/abc-123", "/ru/station/session/42"])(
    "stays out of the running exam at %s",
    (path) => {
      pathname.value = path;
      const { container } = render(<KioskChrome />);

      expect(container).toBeEmptyDOMElement();
    },
  );
});
