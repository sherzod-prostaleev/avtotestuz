FROM python:3.12-slim@sha256:57cd7c3a7a273101a6485ba99423ee568157882804b1124b4dd04266317710de

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY watcher.py .

RUN useradd --uid 10001 --create-home --shell /usr/sbin/nologin watcher \
 && mkdir -p /data \
 && chown watcher:watcher /data

ENV PYTHONUNBUFFERED=1
ENV HUMO_QUEUE_DB=/data/humo-queue.sqlite3
USER watcher:watcher
CMD ["python", "-u", "watcher.py"]
