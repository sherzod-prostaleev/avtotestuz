#!/usr/bin/env python3
"""Fail when direct Humo pins and the generated hashed lock diverge."""

from __future__ import annotations

from pathlib import Path
import re


ROOT = Path(__file__).resolve().parent
PIN = re.compile(r"^([A-Za-z0-9_.-]+)==([^\s\\]+)")


def normalize(name: str) -> str:
    return re.sub(r"[-_.]+", "-", name).lower()


def pins(path: Path) -> dict[str, str]:
    result: dict[str, str] = {}
    for line_number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#") or line.startswith("--hash="):
            continue
        match = PIN.match(line)
        if match is None:
            raise SystemExit(f"{path.name}:{line_number}: exact name==version pin required")
        name, version = normalize(match.group(1)), match.group(2)
        if name in result:
            raise SystemExit(f"{path.name}:{line_number}: duplicate package {name}")
        result[name] = version
    return result


def main() -> None:
    direct = pins(ROOT / "requirements.txt")
    locked = pins(ROOT / "requirements.lock")
    mismatches = [
        f"{name}: direct={version}, lock={locked.get(name, 'missing')}"
        for name, version in sorted(direct.items())
        if locked.get(name) != version
    ]
    if mismatches:
        raise SystemExit("requirements.lock is stale: " + "; ".join(mismatches))
    lock_text = (ROOT / "requirements.lock").read_text(encoding="utf-8")
    if "--hash=sha256:" not in lock_text:
        raise SystemExit("requirements.lock contains no sha256 hashes")
    print(f"requirements lock aligned: {len(direct)} direct / {len(locked)} total pins")


if __name__ == "__main__":
    main()
