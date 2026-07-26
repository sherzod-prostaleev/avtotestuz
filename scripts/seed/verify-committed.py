#!/usr/bin/env python3
"""Verify committed content seeds match the live product counts.

Canonical numbers (post-dedupe / 62-bilet catalog):
  - questions: 1231
  - variants: 62
  - explanations: 1219
  - sign groups: 7
  - signs: 285

Fail loudly when docs, FE badges, or a partial regenerate drift again.
"""

from __future__ import annotations

import json
import sys
from collections import Counter
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
AVTO = ROOT / "backend" / "seed" / "avtoimtihon" / "data.json"
SIGNS = ROOT / "backend" / "seed" / "signs" / "data.json"
QUESTION_SIGNS = ROOT / "backend" / "seed" / "avtoimtihon" / "question_signs.json"

EXPECT = {
    "questions": 1231,
    "variants": 62,
    "explanations": 1219,
    "sign_groups": 7,
    "signs": 285,
    "categories": 13,
}


def fail(msg: str) -> None:
    print(f"FAIL: {msg}", file=sys.stderr)
    raise SystemExit(1)


def main() -> None:
    if not AVTO.is_file():
        fail(f"missing {AVTO} — committed avtoimtihon corpus required for wipe restore")
    if not SIGNS.is_file():
        fail(f"missing {SIGNS} — run: cd backend && go run ./cmd/gensigns -out seed/signs")

    avto = json.loads(AVTO.read_text(encoding="utf-8"))
    signs = json.loads(SIGNS.read_text(encoding="utf-8"))

    q = avto.get("questions") or []
    v = avto.get("variants") or []
    e = avto.get("explanations") or []
    sg = signs.get("sign_groups") or []
    s = signs.get("signs") or []

    got = {
        "questions": len(q),
        "variants": len(v),
        "explanations": len(e),
        "sign_groups": len(sg),
        "signs": len(s),
        "categories": len({item.get("category") for item in q if item.get("category")}),
    }

    for key, want in EXPECT.items():
        if got[key] != want:
            fail(f"{key}: got {got[key]}, want {want}")

    assigned: set[str] = set()
    for variant in v:
        qs = variant.get("questions") or []
        if len(qs) not in (11, 20):
            # Bilet 62 is the short remainder (11); all others are official 20.
            fail(f"variant {variant.get('number')}: {len(qs)} questions (want 20 or 11)")
        assigned.update(qs)

    orphans = [item["ext_id"] for item in q if item.get("ext_id") not in assigned]
    if orphans:
        fail(f"{len(orphans)} questions not assigned to any variant (e.g. {orphans[:3]})")

    if QUESTION_SIGNS.is_file():
        links = json.loads(QUESTION_SIGNS.read_text(encoding="utf-8"))
        if not isinstance(links, dict):
            fail("question_signs.json must be an object of ext_id → [sign_code]")
        sign_codes = {item.get("code") for item in s if item.get("code")}
        ext_ids = {item.get("ext_id") for item in q if item.get("ext_id")}
        for ext_id, codes in links.items():
            if ext_id not in ext_ids:
                fail(f"question_signs.json unknown question {ext_id}")
            if not isinstance(codes, list):
                fail(f"question_signs.json {ext_id}: codes must be a list")
            for code in codes:
                if code not in sign_codes:
                    fail(f"question_signs.json {ext_id}: unknown sign {code}")

    cats = Counter(item.get("category") for item in q)
    if None in cats or "" in cats:
        fail("every question must have a category")

    print("OK committed seed parity:")
    for key in EXPECT:
        print(f"  {key}={got[key]}")
    print("  categories:", ", ".join(f"{k}:{n}" for k, n in cats.most_common()))


if __name__ == "__main__":
    main()
