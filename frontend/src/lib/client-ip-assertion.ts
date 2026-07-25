import { createHmac } from "node:crypto";
import { isIP } from "node:net";

const MIN_SECRET_BYTES = 32;
const MAX_TRUSTED_PROXY_HOPS = 10;

export const clientIPAssertionHeaders = {
  ip: "X-Avtotest-Client-IP",
  timestamp: "X-Avtotest-Client-IP-Timestamp",
  signature: "X-Avtotest-Client-IP-Signature",
} as const;

function loadConfig(): { secret: string; trustedProxyHops: number } | null {
  const secret = process.env.CLIENT_IP_ASSERTION_SECRET ?? "";
  const rawHops = process.env.TRUSTED_PROXY_HOPS?.trim() ?? "";
  const required = process.env.NODE_ENV === "production";

  if (secret === "" && rawHops === "") {
    if (required) {
      throw new Error("client IP assertions are required in production");
    }
    return null;
  }

  if (secret === "" || rawHops === "") {
    throw new Error("CLIENT_IP_ASSERTION_SECRET and TRUSTED_PROXY_HOPS must be configured together");
  }
  if (Buffer.byteLength(secret, "utf8") < MIN_SECRET_BYTES) {
    throw new Error(`CLIENT_IP_ASSERTION_SECRET must be at least ${MIN_SECRET_BYTES} bytes`);
  }

  const trustedProxyHops = Number(rawHops);
  if (
    !Number.isSafeInteger(trustedProxyHops) ||
    trustedProxyHops < 1 ||
    trustedProxyHops > MAX_TRUSTED_PROXY_HOPS
  ) {
    throw new Error(`TRUSTED_PROXY_HOPS must be an integer between 1 and ${MAX_TRUSTED_PROXY_HOPS}`);
  }

  return { secret, trustedProxyHops };
}

function trustedClientIP(request: Request, trustedProxyHops: number): string {
  const forwardedFor = request.headers.get("x-forwarded-for");
  if (forwardedFor === null) {
    throw new Error("trusted proxy did not provide X-Forwarded-For");
  }

  const chain = forwardedFor.split(",").map((part) => part.trim());
  if (chain.some((part) => part === "") || chain.length < trustedProxyHops) {
    throw new Error("trusted proxy provided an invalid X-Forwarded-For chain");
  }

  // Count from the right so browser-prepended/spoofed values cannot become
  // the client IP when every configured trusted proxy appends its peer.
  const clientIP = chain[chain.length - trustedProxyHops];
  if (isIP(clientIP) === 0) {
    throw new Error("trusted proxy provided an invalid client IP");
  }
  return clientIP;
}

function normalizeAssertionPath(backendPath: string): string {
  if (backendPath.startsWith("/api/v1/")) return backendPath;
  if (backendPath.startsWith("/api/v1")) return backendPath;
  const trimmed = backendPath.startsWith("/") ? backendPath : `/${backendPath}`;
  return `/api/v1${trimmed}`;
}

function signingPayload(timestamp: string, clientIP: string, backendPath: string): string {
  return ["v1", timestamp, clientIP, "POST", normalizeAssertionPath(backendPath)].join("\n");
}

/**
 * Builds a short-lived, server-only assertion for auth rate-limit IP binding.
 *
 * `backendPath` must match the backend route path the BFF will call (e.g.
 * `/auth/login` or `/api/v1/auth/login`) — the HMAC covers method+path so a
 * signature minted for OTP cannot be replayed onto login/register.
 *
 * In development the mechanism is disabled when both env vars are absent, so
 * the backend safely rate-limits the BFF socket address. In production, or
 * whenever either env var is supplied, incomplete/untrusted input throws and
 * the route returns its existing network_error response instead of forwarding.
 */
export function buildClientIPAssertionHeaders(
  request: Request,
  backendPath: string
): Record<string, string> {
  const config = loadConfig();
  if (config === null) return {};

  const clientIP = trustedClientIP(request, config.trustedProxyHops);
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const signature = createHmac("sha256", config.secret)
    .update(signingPayload(timestamp, clientIP, backendPath))
    .digest("base64url");

  return {
    [clientIPAssertionHeaders.ip]: clientIP,
    [clientIPAssertionHeaders.timestamp]: timestamp,
    [clientIPAssertionHeaders.signature]: signature,
  };
}
