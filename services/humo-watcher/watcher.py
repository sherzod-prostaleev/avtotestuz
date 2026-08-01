#!/usr/bin/env python3
"""Listen to HUMOcardbot pushes and forward To'ldirish texts to Driver Go API."""

from __future__ import annotations

import asyncio
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


class DurableSpool:
    """SQLite-backed at-least-once queue keyed by Telegram message id."""

    def __init__(self, path: str) -> None:
        Path(path).parent.mkdir(parents=True, exist_ok=True)
        self.path = path
        with self._connect() as db:
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
        with self._connect() as db:
            db.execute(
                "INSERT OR IGNORE INTO pending_ingest (msg_id, raw_text, created_at) VALUES (?, ?, ?)",
                (msg_id, raw_text, int(time.time())),
            )

    def first(self) -> tuple[int, str] | None:
        with self._connect() as db:
            row = db.execute(
                "SELECT msg_id, raw_text FROM pending_ingest ORDER BY created_at, msg_id LIMIT 1"
            ).fetchone()
        return (int(row[0]), str(row[1])) if row else None

    def ack(self, msg_id: int) -> None:
        with self._connect() as db:
            db.execute("DELETE FROM pending_ingest WHERE msg_id = ?", (msg_id,))


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

    @client.on(events.NewMessage(chats=bot_username))
    async def handler(event: events.NewMessage.Event) -> None:  # type: ignore[name-defined]
        text = (event.raw_text or "").strip()
        if not text:
            return
        # Only deposit notifications; parser on API is authoritative.
        lower = text.lower()
        if "to'ldirish" not in lower and "toldirish" not in lower and "пополн" not in lower:
            return
        await asyncio.to_thread(spool.put, int(event.id), text)
        await drain_spool(session, spool, spool_lock)

    await client.start()
    me = await client.get_me()
    LOG.info("connected as %s; watching @%s", getattr(me, "username", me.id), bot_username)
    await client.run_until_disconnected()


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
    async with aiohttp.ClientSession(timeout=timeout) as session:
        retry_task = asyncio.create_task(retry_spool_loop(session, spool, spool_lock))
        try:
            while True:
                creds = await load_credentials(session)
                if not creds or not creds.get("session"):
                    LOG.info("waiting for admin telegram credentials…")
                    await asyncio.sleep(POLL_CREDS_SEC)
                    continue
                try:
                    await run_client(creds, session, spool, spool_lock)
                except Exception as exc:  # noqa: BLE001
                    LOG.exception("client stopped: %s", exc)
                LOG.info("reconnecting in %ss", POLL_CREDS_SEC)
                await asyncio.sleep(POLL_CREDS_SEC)
        finally:
            retry_task.cancel()
            await asyncio.gather(retry_task, return_exceptions=True)


if __name__ == "__main__":
    asyncio.run(main())
