#!/usr/bin/env python3
"""Listen to HUMOcardbot pushes and forward To'ldirish texts to Driver Go API."""

from __future__ import annotations

import asyncio
from contextlib import closing
import logging
import os
from pathlib import Path
import sqlite3
import sys
import time
from typing import Any

import aiohttp
from telethon import TelegramClient, events
from telethon.sessions import StringSession

LOG = logging.getLogger("humo-watcher")

API_BASE = os.environ.get("API_BASE_URL", "http://api:8080/api/v1").rstrip("/")
INGEST_TOKEN = os.environ.get("MANUAL_PAY_INGEST_TOKEN", "")
POLL_CREDS_SEC = int(os.environ.get("HUMO_CREDS_POLL_SEC", "30"))
QUEUE_DB = os.environ.get("HUMO_QUEUE_DB", "/data/humo-queue.sqlite3")
HEALTH_HOST = os.environ.get("HUMO_HEALTH_HOST", "0.0.0.0")
HEALTH_PORT = int(os.environ.get("HUMO_HEALTH_PORT", "8088"))


async def serve_health(
    reader: asyncio.StreamReader, writer: asyncio.StreamWriter
) -> None:
    """Tiny dependency-free liveness endpoint served by the watcher event loop."""
    try:
        await asyncio.wait_for(reader.read(1024), timeout=2)
        body = b'{"status":"ok"}\n'
        writer.write(
            b"HTTP/1.1 200 OK\r\n"
            b"Content-Type: application/json\r\n"
            b"Cache-Control: no-store\r\n"
            + f"Content-Length: {len(body)}\r\n".encode()
            + b"Connection: close\r\n\r\n"
            + body
        )
        await writer.drain()
    except (ConnectionError, asyncio.TimeoutError):
        pass
    finally:
        writer.close()
        await writer.wait_closed()


class DurableSpool:
    """SQLite-backed at-least-once queue keyed by Telegram message id."""

    def __init__(self, path: str) -> None:
        Path(path).parent.mkdir(parents=True, exist_ok=True)
        self.path = path
        with closing(self._connect()) as db:
            with db:
                db.execute(
                    """CREATE TABLE IF NOT EXISTS pending_ingest (
                           msg_id INTEGER PRIMARY KEY,
                           raw_text TEXT NOT NULL,
                           created_at INTEGER NOT NULL
                       )"""
                )

    def _connect(self) -> sqlite3.Connection:
        db = sqlite3.connect(self.path, timeout=10)
        db.execute("PRAGMA journal_mode=WAL")
        db.execute("PRAGMA synchronous=FULL")
        return db

    def put(self, msg_id: int, raw_text: str) -> None:
        with closing(self._connect()) as db:
            with db:
                db.execute(
                    "INSERT OR IGNORE INTO pending_ingest (msg_id, raw_text, created_at) VALUES (?, ?, ?)",
                    (msg_id, raw_text, int(time.time())),
                )

    def first(self) -> tuple[int, str] | None:
        with closing(self._connect()) as db:
            row = db.execute(
                "SELECT msg_id, raw_text FROM pending_ingest ORDER BY created_at, msg_id LIMIT 1"
            ).fetchone()
        return (int(row[0]), str(row[1])) if row else None

    def ack(self, msg_id: int) -> None:
        with closing(self._connect()) as db:
            with db:
                db.execute("DELETE FROM pending_ingest WHERE msg_id = ?", (msg_id,))


class SpoolWriteError(RuntimeError):
    """A callback could not durably enqueue a payment notification."""

    def __init__(self, cause: Exception) -> None:
        super().__init__("spool write failed")
        self.__cause__ = cause


async def drain_spool(
    session: aiohttp.ClientSession,
    spool: DurableSpool,
    lock: asyncio.Lock,
) -> bool:
    async with lock:
        while True:
            pending = await asyncio.to_thread(spool.first)
            if pending is None:
                return True
            msg_id, raw_text = pending
            try:
                await ingest(session, raw_text, msg_id)
            except Exception as exc:  # noqa: BLE001
                LOG.warning("queued ingest retry failed for msg_id=%s: %s", msg_id, exc)
                return False
            await asyncio.to_thread(spool.ack, msg_id)


async def retry_spool_loop(
    session: aiohttp.ClientSession,
    spool: DurableSpool,
    lock: asyncio.Lock,
) -> None:
    delay = 2
    while True:
        drained = await drain_spool(session, spool, lock)
        delay = 2 if drained else min(delay * 2, 300)
        await asyncio.sleep(delay)


async def fetch_json(session: aiohttp.ClientSession, method: str, path: str, **kwargs: Any) -> dict[str, Any]:
    url = f"{API_BASE}{path}"
    headers = kwargs.pop("headers", {})
    headers["X-Internal-Token"] = INGEST_TOKEN
    async with session.request(method, url, headers=headers, **kwargs) as resp:
        body = await resp.json(content_type=None)
        if resp.status >= 400:
            raise RuntimeError(f"{method} {path} -> {resp.status}: {body}")
        data = body.get("data", body)
        return data if isinstance(data, dict) else {"data": data}


async def load_credentials(session: aiohttp.ClientSession) -> dict[str, Any] | None:
    try:
        return await fetch_json(session, "GET", "/internal/manual-pay/tg-credentials")
    except Exception as exc:  # noqa: BLE001
        LOG.warning("credentials unavailable: %s", exc)
        return None


async def ingest(session: aiohttp.ClientSession, raw_text: str, msg_id: int) -> None:
    out = await fetch_json(
        session,
        "POST",
        "/internal/manual-pay/ingest",
        json={"raw_text": raw_text, "telegram_msg_id": msg_id},
    )
    LOG.info("ingest ok: %s", out)


async def run_client(
    creds: dict[str, Any],
    session: aiohttp.ClientSession,
    spool: DurableSpool,
    spool_lock: asyncio.Lock,
) -> None:
    api_id = int(creds["api_id"])
    api_hash = str(creds["api_hash"])
    string_session = str(creds["session"])
    bot_username = str(creds.get("humo_bot_username") or "HUMOcardbot").lstrip("@")

    client = TelegramClient(StringSession(string_session), api_id, api_hash)
    callback_failure: asyncio.Future[None] = asyncio.get_running_loop().create_future()

    @client.on(events.NewMessage(chats=bot_username))
    async def handler(event: events.NewMessage.Event) -> None:  # type: ignore[name-defined]
        text = (event.raw_text or "").strip()
        if not text:
            return
        # Only deposit notifications; parser on API is authoritative.
        lower = text.lower()
        if "to'ldirish" not in lower and "toldirish" not in lower and "пополн" not in lower:
            return
        try:
            await asyncio.to_thread(spool.put, int(event.id), text)
        except Exception as exc:  # Telethon callback exceptions are otherwise only logged.
            LOG.critical("spool write failed for msg_id=%s", event.id, exc_info=exc)
            if not callback_failure.done():
                callback_failure.set_exception(SpoolWriteError(exc))
            return
        await drain_spool(session, spool, spool_lock)

    await client.start()
    me = await client.get_me()
    LOG.info("connected as %s; watching @%s", getattr(me, "username", me.id), bot_username)
    disconnected = asyncio.create_task(
        client.run_until_disconnected(), name="telegram-disconnected"
    )
    try:
        done, _ = await asyncio.wait(
            (disconnected, callback_failure), return_when=asyncio.FIRST_COMPLETED
        )
        if callback_failure in done:
            await callback_failure
        await disconnected
    finally:
        if not disconnected.done():
            disconnected.cancel()
        await asyncio.gather(disconnected, return_exceptions=True)
        await client.disconnect()


async def watch_credentials_loop(
    session: aiohttp.ClientSession,
    spool: DurableSpool,
    spool_lock: asyncio.Lock,
) -> None:
    """Keep reconnecting the Telegram client without hiding supervisor failure."""
    while True:
        creds = await load_credentials(session)
        if not creds or not creds.get("session"):
            LOG.info("waiting for admin telegram credentials…")
            await asyncio.sleep(POLL_CREDS_SEC)
            continue
        try:
            await run_client(creds, session, spool, spool_lock)
        except SpoolWriteError:
            raise
        except Exception as exc:  # noqa: BLE001
            LOG.exception("client stopped: %s", exc)
        LOG.info("reconnecting in %ss", POLL_CREDS_SEC)
        await asyncio.sleep(POLL_CREDS_SEC)


async def supervise_critical_tasks(*tasks: asyncio.Task[None]) -> None:
    """Fail the process if any forever-task errors or returns unexpectedly."""
    if not tasks:
        raise ValueError("at least one critical task is required")
    try:
        done, _ = await asyncio.wait(tasks, return_when=asyncio.FIRST_COMPLETED)
        for task in done:
            if task.cancelled():
                raise RuntimeError(f"critical task {task.get_name()} was cancelled")
            error = task.exception()
            if error is not None:
                raise RuntimeError(f"critical task {task.get_name()} failed") from error
        names = ", ".join(sorted(task.get_name() for task in done))
        raise RuntimeError(f"critical task exited unexpectedly: {names}")
    finally:
        for task in tasks:
            if not task.done():
                task.cancel()
        await asyncio.gather(*tasks, return_exceptions=True)


async def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
        stream=sys.stdout,
    )
    if not INGEST_TOKEN:
        LOG.error("MANUAL_PAY_INGEST_TOKEN is empty")
        sys.exit(1)

    timeout = aiohttp.ClientTimeout(total=30)
    spool = DurableSpool(QUEUE_DB)
    spool_lock = asyncio.Lock()
    health_server = await asyncio.start_server(serve_health, HEALTH_HOST, HEALTH_PORT)
    async with health_server, aiohttp.ClientSession(timeout=timeout) as session:
        LOG.info("health endpoint listening on %s:%s", HEALTH_HOST, HEALTH_PORT)
        retry_task = asyncio.create_task(
            retry_spool_loop(session, spool, spool_lock), name="retry-spool"
        )
        telegram_task = asyncio.create_task(
            watch_credentials_loop(session, spool, spool_lock), name="telegram-client"
        )
        # Either loop stopping is fatal. Exiting main makes Docker's existing
        # restart policy recover it instead of leaving a green health server
        # beside a dead payment retry worker.
        await supervise_critical_tasks(retry_task, telegram_task)


if __name__ == "__main__":
    asyncio.run(main())
