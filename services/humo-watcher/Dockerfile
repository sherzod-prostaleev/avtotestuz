FROM python:3.12-alpine@sha256:6d43704baacd1bfbe7c295d7f13079d5d8104ed33568873133f8fc69980419df
ARG VCS_REF=unknown
LABEL org.opencontainers.image.source="https://github.com/sherzod-prostaleev/avtotestuz" \
      org.opencontainers.image.revision="$VCS_REF"

# Same bargain as frontend/Dockerfile: the digest pin above makes the build
# reproducible and also stops it picking up an Alpine security fix on its own.
# CVE-2026-14456 (HIGH, openssl 3.5.7-r0 -> 3.5.8-r0) failed the CVE gate on
# 2026-08-27 while python:3.12-alpine still resolved to this very digest, so
# the packages are upgraded here until upstream rebuilds the tag.
#
# Named packages, not a blanket upgrade, so nothing else the base ships moves
# without being asked for.
RUN apk upgrade --no-cache libcrypto3 libssl3

WORKDIR /app
COPY requirements.txt requirements.lock ./
RUN pip install --disable-pip-version-check --no-cache-dir \
      --require-hashes -r requirements.lock
COPY watcher.py healthcheck.py ./

RUN addgroup -g 10001 -S watcher \
 && adduser -u 10001 -S -D -H -s /sbin/nologin -G watcher watcher \
 && mkdir -p /data \
 && chown watcher:watcher /data

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV HUMO_QUEUE_DB=/data/humo-queue.sqlite3
ENV HUMO_HEALTH_PORT=8088
USER watcher:watcher
EXPOSE 8088
HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=4 CMD ["python", "/app/healthcheck.py"]
CMD ["python", "-u", "watcher.py"]
