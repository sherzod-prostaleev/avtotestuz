import type { Page } from "@playwright/test";

/**
 * Injects the BFF auth cookies (`at` / optional `rt`) from env so Playwright
 * can exercise middleware-gated routes without a UI login.
 *
 * - `E2E_AUTH_TOKEN` — access token cookie value (required for auth-gated specs)
 * - `E2E_REFRESH_TOKEN` — optional refresh cookie (`rt`)
 *
 * Never commit real tokens. In GHA, map `secrets.E2E_AUTH_TOKEN` only when an
 * operator has provisioned a disposable staging/dev JWT; absent secret → specs
 * skip and CI stays green.
 *
 * Middleware only checks cookie *presence*; API calls still need a live backend
 * + valid JWT. Session-gate assertions (stay off `/login`) are the supported
 * vertical without seed wipe / full-stack compose.
 */
export async function applyE2EAuthCookies(page: Page): Promise<void> {
  const access = process.env.E2E_AUTH_TOKEN;
  if (!access) {
    throw new Error("applyE2EAuthCookies called without E2E_AUTH_TOKEN");
  }

  const origin =
    process.env.PLAYWRIGHT_BASE_URL ?? `http://localhost:${process.env.PORT || 3000}`;

  // Playwright wants `url` XOR (`domain` + `path`) — use url only.
  const cookies: {
    name: string;
    value: string;
    url: string;
    httpOnly: boolean;
    sameSite: "Lax";
  }[] = [
    {
      name: "at",
      value: access,
      url: origin,
      httpOnly: true,
      sameSite: "Lax",
    },
  ];

  const refresh = process.env.E2E_REFRESH_TOKEN;
  if (refresh) {
    cookies.push({
      name: "rt",
      value: refresh,
      url: origin,
      httpOnly: true,
      sameSite: "Lax",
    });
  }

  await page.context().addCookies(cookies);
}

export function hasE2EAuthToken(): boolean {
  return Boolean(process.env.E2E_AUTH_TOKEN);
}
