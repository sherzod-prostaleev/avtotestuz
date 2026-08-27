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

const contentSecurityPolicy = [
  "default-src 'self'",
  "base-uri 'self'",
  "object-src 'none'",
  "frame-ancestors 'none'",
  "form-action 'self' https://checkout.paycom.uz",
  // React Dev (and some Next.js HMR helpers) need eval() in development.
  // Keep production strict: never allow unsafe-eval there.
  isProd
    ? "script-src 'self' 'unsafe-inline' https://static.cloudflareinsights.com"
    : "script-src 'self' 'unsafe-inline' 'unsafe-eval' https://static.cloudflareinsights.com",
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

const securityHeaders = [
  { key: "Content-Security-Policy", value: contentSecurityPolicy },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  { key: "X-DNS-Prefetch-Control", value: "off" },
  ...(isProd
    ? [{ key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains; preload" }]
    : []),
];

/** @type {import('next').NextConfig} */
const nextConfig = {
  // Required by frontend/Dockerfile (copies .next/standalone into the runner).
  output: "standalone",
  poweredByHeader: false,
  reactStrictMode: true,
  async headers() {
    return [
      { source: "/:path*", headers: securityHeaders },
      {
        source: "/logo-48.webp",
        headers: [{ key: "Cache-Control", value: "public, max-age=31536000, immutable" }],
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
