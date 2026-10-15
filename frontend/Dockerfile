# AvtoTest Next.js frontend — staging/production image (standalone output).
# BACKEND_URL is a runtime env (server-side BFF → Go API). Do not bake secrets.
#
# Build:  docker build -t avtotest-web:local -f frontend/Dockerfile frontend/
# Run:    see deploy/README.md

FROM node:26-alpine@sha256:aadf416b2cdce311a8811ba3f0608a61b77dbf997500e2eafe781b51f6a0b019 AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM node:26-alpine@sha256:aadf416b2cdce311a8811ba3f0608a61b77dbf997500e2eafe781b51f6a0b019 AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# Standalone emits .next/standalone (+ static/public copied in runner).
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:26-alpine@sha256:aadf416b2cdce311a8811ba3f0608a61b77dbf997500e2eafe781b51f6a0b019 AS runner
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
# Take Alpine's OpenSSL fixes even when the node:24-alpine tag has not been
# rebuilt yet. CVE-2026-14456 (HIGH, openssl 3.5.7-r0 -> 3.5.8-r0) failed the
# container-build CVE gate on 2026-08-27 while `node:24-alpine` still resolved
# to the very digest pinned above -- the image had not changed at all, Trivy's
# database had. Pinning the base by digest is what makes a build reproducible;
# it is also what stops it from silently picking up a distro security fix, and
# this line is the other half of that bargain.
#
# Named packages, not a blanket `apk upgrade`: a full upgrade would quietly
# change anything the base ships, which is the reproducibility the digest pin
# exists to protect. Add a package here when a gate names it, not before.
RUN apk upgrade --no-cache libcrypto3 libssl3

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
