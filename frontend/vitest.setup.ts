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

// Session chip navigators call scrollIntoView; jsdom leaves it undefined.
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = () => {};
}

// @tanstack/react-virtual measures its scroll container via offsetHeight, which
// jsdom always reports as 0 (it has no layout engine). Without a nonzero size the
// virtualizer's range calculation short-circuits to `null` and renders zero rows,
// which has nothing to do with component correctness — it's purely a test-env gap.
// Stub a generous constant so virtualized lists actually mount their rows under test.
if (!Object.getOwnPropertyDescriptor(HTMLElement.prototype, "__mockedOffsetSize")) {
  Object.defineProperty(HTMLElement.prototype, "offsetHeight", {
    configurable: true,
    get: () => 600,
  });
  Object.defineProperty(HTMLElement.prototype, "offsetWidth", {
    configurable: true,
    get: () => 800,
  });
  Object.defineProperty(HTMLElement.prototype, "__mockedOffsetSize", {
    configurable: true,
    value: true,
  });
}
