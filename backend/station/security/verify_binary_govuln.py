#!/usr/bin/env python3
"""Fail on newly disclosed vulnerabilities in the shipped Win7 binary."""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ID = re.compile(r"GO-\d{4}-\d+")


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: verify_binary_govuln.py GOVULNCHECK_JSON ALLOWLIST", file=sys.stderr)
        return 2
    report, allowlist = map(Path, sys.argv[1:])
    allowed = {
        line.strip()
        for line in allowlist.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    }
    if not allowed or any(not ID.fullmatch(item) for item in allowed):
        print("invalid or empty Win7 vulnerability allowlist", file=sys.stderr)
        return 2
    observed: set[str] = set()
    try:
        payload = report.read_text(encoding="utf-8")
        decoder = json.JSONDecoder()
        offset = 0
        while offset < len(payload):
            while offset < len(payload) and payload[offset].isspace():
                offset += 1
            if offset == len(payload):
                break
            event, offset = decoder.raw_decode(payload, offset)
            finding = event.get("finding")
            if isinstance(finding, dict) and isinstance(finding.get("osv"), str):
                observed.add(finding["osv"])
    except (OSError, json.JSONDecodeError) as exc:
        print(f"invalid govulncheck report: {exc}", file=sys.stderr)
        return 2
    if not observed:
        print("govulncheck report contained no findings; refusing silent scanner failure", file=sys.stderr)
        return 2
    new = sorted(observed - allowed)
    stale = sorted(allowed - observed)
    if stale:
        print("stale allowlist entries (review and remove): " + ", ".join(stale), file=sys.stderr)
    if new:
        print("new unreviewed Win7 binary vulnerabilities: " + ", ".join(new), file=sys.stderr)
        return 1
    print(f"Win7 binary vulnerability exception gate: {len(observed)} observed IDs reviewed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
