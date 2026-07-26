#!/usr/bin/env python3
"""One-time helper: login with phone + code and print Telethon StringSession.

Usage (on your laptop, not on the server):
  pip install telethon==1.36.0
  python make_session.py

Then paste the printed session string into Admin → Manual Humo → session string.
Keep this phone's Telegram account logged into @HUMOcardbot.
"""

from __future__ import annotations

from telethon.sync import TelegramClient
from telethon.sessions import StringSession


def main() -> None:
    api_id = int(input("api_id (from https://my.telegram.org/apps): ").strip())
    api_hash = input("api_hash: ").strip()
    with TelegramClient(StringSession(), api_id, api_hash) as client:
        client.start()
        print("\n=== SESSION STRING (copy everything below) ===\n")
        print(client.session.save())
        print("\n=== END ===")


if __name__ == "__main__":
    main()
