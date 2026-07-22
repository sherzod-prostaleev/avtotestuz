import "@testing-library/jest-dom/vitest";
import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";

// @testing-library/react only auto-registers its afterEach(cleanup) when it
// detects a global `afterEach` (i.e. Vitest's `test.globals: true`). This
// project keeps globals off and imports test APIs explicitly, so cleanup
// must be wired up here or DOM nodes leak across tests within a file.
afterEach(() => {
  cleanup();
});

// next-themes (and other libraries) call matchMedia even when enableSystem
// is false; jsdom doesn't implement it, so every test using ThemeProvider
// would throw without this shim.
if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}
