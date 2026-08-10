FROM python:3.12-alpine@sha256:6d43704baacd1bfbe7c295d7f13079d5d8104ed33568873133f8fc69980419df
ARG VCS_REF=unknown
LABEL org.opencontainers.image.source="https://github.com/sherzod-prostaleev/avtotestuz" \
      org.opencontainers.image.revision="$VCS_REF"

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
