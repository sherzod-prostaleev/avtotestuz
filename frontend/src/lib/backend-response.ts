export class InvalidBackendResponseError extends Error {
  constructor() {
    super("backend returned an invalid JSON response");
    this.name = "InvalidBackendResponseError";
  }
}

export async function readBackendJson<T = unknown>(response: Response): Promise<T> {
  const text = await response.text();
  if (!text.trim()) {
    throw new InvalidBackendResponseError();
  }

  try {
    return JSON.parse(text) as T;
  } catch {
    throw new InvalidBackendResponseError();
  }
}

export function extractTokenPair(value: unknown): { accessToken: string; refreshToken: string } {
  if (!value || typeof value !== "object") {
    throw new InvalidBackendResponseError();
  }

  const data = (value as { data?: unknown }).data;
  if (!data || typeof data !== "object") {
    throw new InvalidBackendResponseError();
  }

  const accessToken = (data as { access_token?: unknown }).access_token;
  const refreshToken = (data as { refresh_token?: unknown }).refresh_token;
  if (typeof accessToken !== "string" || !accessToken || typeof refreshToken !== "string" || !refreshToken) {
    throw new InvalidBackendResponseError();
  }

  return { accessToken, refreshToken };
}
