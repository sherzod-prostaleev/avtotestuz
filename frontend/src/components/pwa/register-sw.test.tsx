import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { render } from "@testing-library/react";
import { RegisterServiceWorker } from "./register-sw";

/**
 * The kiosk must never get a service worker.
 *
 * A classroom PC is served from 127.0.0.1 by the local station agent. That is a
 * secure context, so the SW registers and takes over navigation — and the
 * moment the agent is not answering, it renders offline.html: "Internet aloqasi
 * yo'q", with a retry button that shows the same page again. The internet is
 * usually fine on those machines; the agent is what is gone. Sending a driving
 * school to check its router instead of its own software is the failure this
 * test exists to prevent.
 */
describe("RegisterServiceWorker", () => {
  const realLocation = window.location;
  let register: ReturnType<typeof vi.fn>;
  let getRegistrations: ReturnType<typeof vi.fn>;

  const setLocation = (href: string) => {
    Object.defineProperty(window, "location", {
      value: new URL(href),
      writable: true,
      configurable: true,
    });
  };

  beforeEach(() => {
    vi.stubEnv("NODE_ENV", "production");
    register = vi.fn().mockResolvedValue(undefined);
    getRegistrations = vi.fn().mockResolvedValue([]);
    Object.defineProperty(navigator, "serviceWorker", {
      value: { register, getRegistrations },
      writable: true,
      configurable: true,
    });
    Object.defineProperty(globalThis, "caches", {
      value: { keys: vi.fn().mockResolvedValue([]), delete: vi.fn() },
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    Object.defineProperty(window, "location", {
      value: realLocation,
      writable: true,
      configurable: true,
    });
  });

  it("registers on the real site", () => {
    setLocation("https://drivergo.uz/uz-Latn/dashboard");
    render(<RegisterServiceWorker />);
    expect(register).toHaveBeenCalledWith("/sw.js", { scope: "/" });
  });

  it.each([
    "http://127.0.0.1:17817/station",
    "http://127.0.0.1:17817/uz-Latn/station",
    "http://127.0.0.1:17817/uz-Latn/station/tickets",
    "http://localhost:17817/uz-Cyrl/station/session/start",
  ])("does not register on the classroom kiosk (%s)", (href) => {
    setLocation(href);
    render(<RegisterServiceWorker />);
    expect(register).not.toHaveBeenCalled();
  });

  it("still registers for a learner browsing localhost outside the kiosk", () => {
    // A developer or a self-hosted learner on localhost is not a classroom PC;
    // only the /station tree belongs to the agent.
    setLocation("http://localhost:3000/uz-Latn/dashboard");
    render(<RegisterServiceWorker />);
    expect(register).toHaveBeenCalled();
  });
});
