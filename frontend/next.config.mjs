import createNextIntlPlugin from "next-intl/plugin";

const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");

const isProd = process.env.NODE_ENV === "production";
// Local `next dev` has no nginx /media proxy. Same-origin /media/... is rewritten
// to MinIO so CSP img-src 'self' can load real question diagrams. Production nginx
// already proxies /media before Next; set MEDIA_REWRITE_DESTINATION only if a
// containerized Next must reach MinIO itself (e.g. http://minio:9000/media).
const mediaRewriteDestination = (
  process.env.MEDIA_REWRITE_DESTINATION || "http://127.0.0.1:9000/media"
).replace(/\/$/, "");

// One directive list, two policies: only frame-ancestors differs between the
// learner app and the admin panel, so they cannot drift apart.
const buildContentSecurityPolicy = (frameAncestors) => [
  "default-src 'self'",
  "base-uri 'self'",
  "object-src 'none'",
  frameAncestors,
  "form-action 'self' https://checkout.paycom.uz",
  // React Dev (and some Next.js HMR helpers) need eval() in development.
  // Keep production strict: never allow unsafe-eval there.
  isProd
    ? "script-src 'self' 'unsafe-inline' https://static.cloudflareinsights.com https://telegram.org"
    : "script-src 'self' 'unsafe-inline' 'unsafe-eval' https://static.cloudflareinsights.com https://telegram.org",
  "style-src 'self' 'unsafe-inline'",
  // Production: same-origin /media (nginx) + https CDNs.
  // Development: also allow the raw MinIO ports for leftover absolute URLs
  // (signs/saved/demo still pass API image_url through without the session resolver).
  isProd
    ? "img-src 'self' data: blob: https:"
    : "img-src 'self' data: blob: https: http://localhost:9000 http://127.0.0.1:9000",
  "font-src 'self' data:",
  "connect-src 'self' https: wss:",
  "worker-src 'self' blob:",
  "manifest-src 'self'",
  ...(isProd ? ["upgrade-insecure-requests"] : []),
].join("; ");

// Telegram Web runs Mini Apps in an iframe; the mobile/desktop clients use a
// webview and need nothing.
const contentSecurityPolicy = buildContentSecurityPolicy(
  "frame-ancestors 'self' https://web.telegram.org",
);
// A later matching headers() entry replaces an earlier one with the same key,
// so admin needs the FULL policy with frame-ancestors 'none', not just that
// directive, or it would lose script-src and the rest.
const adminContentSecurityPolicy = buildContentSecurityPolicy("frame-ancestors 'none'");

const securityHeaders = [
  { key: "Content-Security-Policy", value: contentSecurityPolicy },
  { key: "X-Content-Type-Options", value: "nosniff" },
  // No X-Frame-Options here: it cannot express an allow-list, and CSP
  // frame-ancestors supersedes it in every browser we support.
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  { key: "X-DNS-Prefetch-Control", value: "off" },
  ...(isProd
    ? [{ key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains; preload" }]
    : []),
];

const adminFrameHeaders = [
  { key: "Content-Security-Policy", value: adminContentSecurityPolicy },
  { key: "X-Frame-Options", value: "DENY" },
];

/** @type {import('next').NextConfig} */
const nextConfig = {
  // Required by frontend/Dockerfile (copies .next/standalone into the runner).
  output: "standalone",
  poweredByHeader: false,
  reactStrictMode: true,
  experimental: {
    // Next treats a prefetched dynamic route as stale the moment it arrives
    // (staleTimes.dynamic defaults to 0), so the sidebar's prefetches were
    // thrown away and every click still paid the round trip. Keeping them for
    // 30s makes a click a cache read. Safe here because (app) pages are client
    // components that fetch their own data on mount — only the shell is reused,
    // and the effects re-run, so nothing shown to the learner goes stale.
    staleTimes: { dynamic: 30 },
  },
  async headers() {
    return [
      { source: "/:path*", headers: securityHeaders },
      // Must stay after the global entry so these override its CSP.
      { source: "/:locale/admin/:path*", headers: adminFrameHeaders },
      { source: "/api/admin/:path*", headers: adminFrameHeaders },
      {
        source: "/logo-48.webp",
        headers: [{ key: "Cache-Control", value: "public, max-age=31536000, immutable" }],
      },
      {
        // Defense in depth alongside robots.ts' /api/ disallow: these are JSON
        // endpoints, never a search result.
        source: "/api/:path*",
        headers: [{ key: "X-Robots-Tag", value: "noindex, nofollow" }],
      },
    ];
  },
  async rewrites() {
    if (isProd && !process.env.MEDIA_REWRITE_DESTINATION) {
      return [];
    }
    return [
      {
        source: "/media/:path*",
        destination: `${mediaRewriteDestination}/:path*`,
      },
    ];
  },
};

export default withNextIntl(nextConfig);
