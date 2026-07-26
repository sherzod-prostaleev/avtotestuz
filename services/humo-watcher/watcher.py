#!/usr/bin/env python3
"""Listen to HUMOcardbot pushes and forward To'ldirish texts to Driver Go API."""

from __future__ import annotations

import asyncio
import logging
import os
import sys
from typing import Any

import aiohttp
from telethon import TelegramClient, events
from telethon.sessions import StringSession

LOG = logging.getLogger("humo-watcher")

API_BASE = os.environ.get("API_BASE_URL", "http://api:8080/api/v1").rstrip("/")
INGEST_TOKEN = os.environ.get("MANUAL_PAY_INGEST_TOKEN", "")
POLL_CREDS_SEC = int(os.environ.get("HUMO_CREDS_POLL_SEC", "30"))


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


async def run_client(creds: dict[str, Any], session: aiohttp.ClientSession) -> None:
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
        try:
            await ingest(session, text, int(event.id))
        except Exception as exc:  # noqa: BLE001
            LOG.exception("ingest failed: %s", exc)

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
    async with aiohttp.ClientSession(timeout=timeout) as session:
        while True:
            creds = await load_credentials(session)
            if not creds or not creds.get("session"):
                LOG.info("waiting for admin telegram credentials…")
                await asyncio.sleep(POLL_CREDS_SEC)
                continue
            try:
                await run_client(creds, session)
            except Exception as exc:  # noqa: BLE001
                LOG.exception("client stopped: %s", exc)
            LOG.info("reconnecting in %ss", POLL_CREDS_SEC)
            await asyncio.sleep(POLL_CREDS_SEC)


if __name__ == "__main__":
    asyncio.run(main())
