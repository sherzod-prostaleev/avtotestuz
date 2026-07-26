#!/usr/bin/env python3
"""Rebuild question_signs.json from an Onless sign→question scrape.

Input: /tmp/onless-sign-questions.json
  { "<sign_code>": [ {"id": "...", "text_uz": "...", "codes": [...] }, ... ] }

Matches Onless question texts to local avtoimtihon questions by normalized
uz-Latn text (exact, then fuzzy), then writes ext_id → [sign_code].
"""

from __future__ import annotations

import json
import re
import sys
from collections import defaultdict
from pathlib import Path

try:
    from rapidfuzz import fuzz, process
except ImportError:  # pragma: no cover
    fuzz = None  # type: ignore
    process = None  # type: ignore
    from difflib import SequenceMatcher

ROOT = Path(__file__).resolve().parents[2]
AVTO = ROOT / "backend/seed/avtoimtihon/data.json"
SIGNS = ROOT / "backend/seed/signs/data.json"
OUT = ROOT / "backend/seed/avtoimtihon/question_signs.json"
ONLESS = Path("/tmp/onless-sign-questions.json")

MARKING_RE = re.compile(
    r"yo'?l chizig|chiziq nimani|uzuq-uzuq chiziq|uzug-uzug|"
    r"1\.1 yoki 1\.11|1\.11 chizi|bo'?ylama tasma",
    re.I,
)


def norm(s: str) -> str:
    s = (s or "").lower()
    for a, b in (
        ("ʻ", "'"),
        ("`", "'"),
        ("’", "'"),
        ("ʻ", "'"),
        ("ў", "o'"),
        ("ғ", "g'"),
        ("қ", "q"),
        ("ҳ", "h"),
        ("оʻ", "o'"),
        ("gʻ", "g'"),
        ("oʻ", "o'"),
        ("«", " "),
        ("»", " "),
        ("“", " "),
        ("”", " "),
        ("—", " "),
        ("–", " "),
    ):
        s = s.replace(a, b)
    # synonym normalizations seen across banks
    s = s.replace("moslama", "qurilma")
    s = s.replace("yaqinlashayotganlik", "yaqinlashganlik")
    s = s.replace("ushbu ogohlantiruvchi belgilardan qaysilari", "qaysi belgi")
    s = s.replace("ogohlantiruvchi belgilardan qaysilari", "qaysi belgi")
    s = re.sub(r"[\"'.,:;!?()\[\]{}]+", " ", s)
    s = re.sub(r"\s+", " ", s).strip()
    return s


def fuzzy_match(
    key: str,
    choices: list[str],
    text_to_ids: dict[str, list[str]],
    threshold: float = 78.0,
) -> list[str]:
    """Return ext_ids whose text is close enough to key (0–100 score scale)."""
    if not key or not choices:
        return []
    if process is not None:
        hit = process.extractOne(
            key,
            choices,
            scorer=fuzz.token_set_ratio,
            score_cutoff=threshold,
        )
        if not hit:
            return []
        text, score, _ = hit
        # Prefer token_set, but require a second opinion for mid-range scores so
        # loosely related questions do not all collapse onto one popular stem.
        if score < 90:
            partial = fuzz.partial_ratio(key, text)
            if partial < 82:
                return []
        return list(dict.fromkeys(text_to_ids.get(text, [])))

    # Slow fallback without rapidfuzz.
    best_score = -1.0
    best_text = ""
    for text in choices:
        score = SequenceMatcher(None, key, text).ratio() * 100
        if score > best_score:
            best_score = score
            best_text = text
    if best_score < threshold or not best_text:
        return []
    return list(dict.fromkeys(text_to_ids.get(best_text, [])))


def main() -> int:
    if not ONLESS.exists():
        print(f"missing {ONLESS}", file=sys.stderr)
        return 1

    onless = json.loads(ONLESS.read_text(encoding="utf-8"))
    catalog = {s["code"] for s in json.loads(SIGNS.read_text(encoding="utf-8"))["signs"]}
    questions = json.loads(AVTO.read_text(encoding="utf-8"))["questions"]

    by_text: dict[str, list[str]] = defaultdict(list)
    for q in questions:
        uz = norm((q.get("texts") or {}).get("uz-Latn") or "")
        if uz:
            by_text[uz].append(q["ext_id"])
    choices = list(by_text.keys())

    # Prefer git HEAD mapping as the "old" baseline when present.
    old = json.loads(OUT.read_text(encoding="utf-8")) if OUT.exists() else {}
    scraped_codes = {c for c in onless if c in catalog}

    rebuilt: dict[str, list[str]] = defaultdict(list)
    matched_exact = matched_fuzzy = 0
    unmatched_texts: list[tuple[str, str]] = []

    for code, rows in onless.items():
        if code not in catalog:
            continue
        for row in rows:
            text = row.get("text_uz") or ""
            key = norm(text)
            ids = list(by_text.get(key) or [])
            if ids:
                matched_exact += 1
                # One Onless question → one local question (prefer first stable id).
                ids = [sorted(ids)[0]]
            else:
                ids = fuzzy_match(key, choices, by_text)
                if ids:
                    matched_fuzzy += 1
                    ids = [ids[0]]
                else:
                    unmatched_texts.append((code, text[:160]))
                    continue
            for ext in ids:
                if code not in rebuilt[ext]:
                    rebuilt[ext].append(code)

    # Full replace from Onless for scraped catalog signs. Do not keep the old
    # heuristic map for those codes — that is what produced 1.1↔marking bugs.
    cleaned: dict[str, list[str]] = {}
    for ext, codes in rebuilt.items():
        q = next((x for x in questions if x["ext_id"] == ext), None)
        uz = ((q.get("texts") or {}).get("uz-Latn") if q else "") or ""
        keep = []
        for c in sorted(set(codes)):
            if c.startswith("1.") and MARKING_RE.search(uz) and "belgi" not in uz.lower():
                continue
            keep.append(c)
        if keep:
            cleaned[ext] = keep
    _ = old  # baseline intentionally unused after Onless full scrape

    OUT.write_text(
        json.dumps(cleaned, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )

    by_sign: dict[str, int] = defaultdict(int)
    for codes in cleaned.values():
        for c in codes:
            by_sign[c] += 1

    print(f"wrote {OUT} — {len(cleaned)} questions linked")
    print(f"exact={matched_exact} fuzzy={matched_fuzzy} unmatched={len(unmatched_texts)}")
    print(f"scraped sign codes applied: {len(scraped_codes)}")
    if "1.1" in onless:
        print("1.1 links:")
        for row in onless["1.1"]:
            key = norm(row.get("text_uz") or "")
            ids = by_text.get(key) or fuzzy_match(key, choices, by_text)
            print(f"  {ids or ['UNMATCHED']} :: {(row.get('text_uz') or '')[:120]}")
        print(f"1.1 local count: {by_sign.get('1.1', 0)}")
        # invert
        for ext, codes in cleaned.items():
            if "1.1" in codes:
                q = next(x for x in questions if x["ext_id"] == ext)
                print("  ->", ext, (q["texts"].get("uz-Latn") or "")[:100])

    for code, text in unmatched_texts[:15]:
        print(f"UNMATCHED [{code}] {text}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
