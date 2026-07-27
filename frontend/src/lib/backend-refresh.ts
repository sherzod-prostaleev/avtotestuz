import { backendFetch } from "@/lib/backend";
import { extractTokenPair, readBackendJson } from "@/lib/backend-response";
import type { RefreshedTokens } from "@/lib/refresh-lock";

/**
 * Calls Go /auth/refresh.
 * - Success → token pair
 * - Definitive auth failure (4xx) → null (caller may clear cookies)
 * - Upstream 5xx / unexpected → throws (caller must NOT clear cookies)
 */
export async function callBackendRefresh(refreshToken: string): Promise<RefreshedTokens | null> {
  const res = await backendFetch("/auth/refresh", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
  if (res.status >= 500) {
    throw new Error("refresh upstream unavailable");
  }
  if (!res.ok) return null;
  return extractTokenPair(await readBackendJson(res));
}
