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

// Spec files run in parallel (tests inside one file stay serial): every spec
// stubs its own /api routes and gets its own browser context, so files share
// nothing but the dev server. One worker made the suite take ~2.5 minutes for
// no isolation benefit. PW_WORKERS overrides; GitHub's runners have 4 cores.
const WORKERS = Number(process.env.PW_WORKERS) || (process.env.GITHUB_ACTIONS ? 4 : 8);

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: WORKERS,
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
    // /checkout/done/<bot> links back only to the configured bot.
    env: { TELEGRAM_BOT_USERNAME: process.env.TELEGRAM_BOT_USERNAME ?? "DriverGouzBot" },
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
