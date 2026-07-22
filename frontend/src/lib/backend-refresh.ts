import { backendFetch } from "@/lib/backend";
import type { RefreshedTokens } from "@/lib/refresh-lock";

export async function callBackendRefresh(refreshToken: string): Promise<RefreshedTokens | null> {
  const res = await backendFetch("/auth/refresh", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
  if (!res.ok) return null;
  const data = await res.json();
  return { accessToken: data.data.access_token, refreshToken: data.data.refresh_token };
}
