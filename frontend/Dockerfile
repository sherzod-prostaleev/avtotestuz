# AvtoTest Next.js frontend — staging/production image (standalone output).
# BACKEND_URL is a runtime env (server-side BFF → Go API). Do not bake secrets.
#
# Build:  docker build -t avtotest-web:local -f frontend/Dockerfile frontend/
# Run:    see deploy/README.md

FROM node:24-alpine@sha256:d32cdf619f63fe0471182d08996dd516c6275bb5fd31ae06e55a570bd9e1ad43 AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM node:24-alpine@sha256:d32cdf619f63fe0471182d08996dd516c6275bb5fd31ae06e55a570bd9e1ad43 AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# Standalone emits .next/standalone (+ static/public copied in runner).
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:24-alpine@sha256:d32cdf619f63fe0471182d08996dd516c6275bb5fd31ae06e55a570bd9e1ad43 AS runner
ARG VCS_REF=unknown
LABEL org.opencontainers.image.source="https://github.com/sherzod-prostaleev/avtotestuz" \
      org.opencontainers.image.revision="$VCS_REF"
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
# The standalone server needs only the `node` binary. Removing package-manager
# toolchains shrinks the runtime attack surface and prevents build-only npm,
# Corepack, Yarn, tar, undici, and brace-expansion advisories from shipping in
# the production container.
RUN rm -rf \
      /usr/local/lib/node_modules/npm \
      /usr/local/lib/node_modules/corepack \
      /opt/yarn-v* \
      /usr/local/bin/npm \
      /usr/local/bin/npx \
      /usr/local/bin/corepack \
      /usr/local/bin/yarn \
      /usr/local/bin/yarnpkg \
      /usr/local/bin/pnpm \
      /usr/local/bin/pnpx \
  && addgroup -S nextjs \
  && adduser -S nextjs -G nextjs
COPY --from=build /app/public ./public
COPY --from=build --chown=nextjs:nextjs /app/.next/standalone ./
COPY --from=build --chown=nextjs:nextjs /app/.next/static ./.next/static
# Multi-process entrypoint. Must land after the standalone copy, which is what
# puts the server.js it wraps at /app/server.js. WEB_WORKERS=1 makes it delegate
# straight to that file, so this is a no-op unless the count is raised.
COPY --from=build --chown=nextjs:nextjs /app/cluster.js ./cluster.js
USER nextjs
EXPOSE 3000
CMD ["node", "cluster.js"]
