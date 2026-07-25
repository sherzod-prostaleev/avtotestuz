import { defineConfig, devices } from "@playwright/test";

const PORT = Number(process.env.PORT) || 3000;
const BASE_URL = `http://localhost:${PORT}`;

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
    trace: "on-first-retry",
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
