#!/usr/bin/env python3
"""Container health probe for the Humo watcher event loop."""

from __future__ import annotations

import json
import os
import sys
from urllib.request import urlopen


def main() -> int:
    port = os.environ.get("HUMO_HEALTH_PORT", "8088")
    try:
        with urlopen(f"http://127.0.0.1:{port}/healthz", timeout=3) as response:
            payload = json.load(response)
            if response.status == 200 and payload.get("status") == "ok":
                return 0
    except (OSError, ValueError):
        return 1
    return 1


if __name__ == "__main__":
    sys.exit(main())
