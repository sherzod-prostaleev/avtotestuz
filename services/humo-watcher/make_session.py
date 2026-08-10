#!/usr/bin/env python3
"""One-time helper: login with phone + code and print Telethon StringSession.

Usage (fish/bash — do not use system pip):
  python -m venv .venv
  .venv/bin/pip install --require-hashes -r requirements.lock
  .venv/bin/python make_session.py

Paste the printed session into Admin → Manual Humo → session string.
Keep this phone logged into @HUMOcardbot.
"""

from __future__ import annotations

import asyncio

from telethon import TelegramClient
from telethon.sessions import StringSession


async def _run(api_id: int, api_hash: str) -> str:
    client = TelegramClient(StringSession(), api_id, api_hash)
    await client.connect()
    if not await client.is_user_authorized():
        phone = input("Phone (+998...): ").strip()
        await client.send_code_request(phone)
        code = input("Code from Telegram/SMS: ").strip()
        try:
            await client.sign_in(phone=phone, code=code)
        except Exception:
            # 2FA password accounts
            password = input("2FA password (if asked): ").strip()
            await client.sign_in(password=password)
    session = client.session.save()
    await client.disconnect()
    return session


def main() -> None:
    api_id = int(input("api_id (from https://my.telegram.org/apps): ").strip())
    api_hash = input("api_hash: ").strip()
    session = asyncio.run(_run(api_id, api_hash))
    print("\n=== SESSION STRING (copy everything below) ===\n")
    print(session)
    print("\n=== END ===")


if __name__ == "__main__":
    main()
