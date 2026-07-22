import "@testing-library/jest-dom/vitest";

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
