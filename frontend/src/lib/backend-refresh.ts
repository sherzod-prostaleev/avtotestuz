import { backendFetch } from "@/lib/backend";
import { extractTokenPair, readBackendJson } from "@/lib/backend-response";
import type { RefreshedTokens } from "@/lib/refresh-lock";

export async function callBackendRefresh(refreshToken: string): Promise<RefreshedTokens | null> {
  const res = await backendFetch("/auth/refresh", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
  if (!res.ok) return null;
  return extractTokenPair(await readBackendJson(res));
}
