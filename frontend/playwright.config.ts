import { defineConfig, devices } from "@playwright/test";

const PORT = Number(process.env.PORT) || 3000;
const BASE_URL = `http://localhost:${PORT}`;
const AUTH_SECRETS_PRESENT = Boolean(
  process.env.E2E_AUTH_TOKEN || process.env.E2E_REFRESH_TOKEN,
);

// Optional auth-gated specs (see e2e/helpers/auth.ts):
//   E2E_AUTH_TOKEN     → sets httpOnly `at` cookie (session-gate smoke)
//   E2E_REFRESH_TOKEN  → optional `rt` cookie
// Never commit real tokens; GHA maps secrets when present, else specs skip.

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: "list",
  use: {
    baseURL: BASE_URL,
    // Playwright traces retain network headers/cookies. Never record one when
    // CI supplied a real access or refresh token.
    trace: AUTH_SECRETS_PRESENT ? "off" : "on-first-retry",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: "npm run dev",
    port: PORT,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
