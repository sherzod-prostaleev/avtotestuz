#!/usr/bin/env python3
"""Extract structured LegalRefs from explanation prose into seed data.json.

Patterns cover Uzbek/Russian YHQ/ПДД citations that already appear inline.
Writes legal_refs as [{"code": "...", "title": "..."}] per explanation.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DATA = ROOT / "backend/seed/avtoimtihon/data.json"

# Ordered: more specific first.
PATTERNS: list[tuple[re.Pattern[str], str]] = [
    (
        re.compile(
            r"(?:YHQ|ЙҲҚ)\s*(\d+)\s*[-–]?\s*bob(?:i|iga)?\s*(\d+)\s*[-–]?\s*band(?:i|iga)?",
            re.I,
        ),
        "YHQ {0}-bob {1}-band",
    ),
    (
        re.compile(
            r"(?:YHQ|ЙҲҚ)\s*(?:ning\s+)?(\d+)\s*[-–]?\s*(?:modda|модда)(?:si|siga)?",
            re.I,
        ),
        "YHQ {0}-modda",
    ),
    (
        re.compile(
            r"ПДД[,\s]+(?:глава|гл\.)\s*(\d+)[,\s]+(?:пункт|п\.)\s*(\d+)",
            re.I,
        ),
        "ПДД гл.{0} п.{1}",
    ),
    (
        re.compile(
            r"Пункта?\s+(\d+)\s+главы\s+(\d+)\s+ПДД",
            re.I,
        ),
        "ПДД гл.{1} п.{0}",
    ),
    (
        re.compile(
            r"Приложени[ея]\s*№?\s*(\d+)\s*(?:к\s+)?ПДД(?:\s+пункт\s+([\d.]+))?",
            re.I,
        ),
        "ПДД Прил.№{0}",
    ),
    (
        re.compile(r"(?:YHQ|ЙҲҚ|ПДД)\s*(?:пункт|п\.|band)\s*([\d.]+)", re.I),
        "YHQ/ПДД п.{0}",
    ),
    (
        re.compile(r"(?:знак|belgi)\s+([\d]+(?:\.[\d]+){1,2})", re.I),
        "Belgi {0}",
    ),
]


def blocks_text(blocks: dict | list | None) -> str:
    if not blocks:
        return ""
    parts: list[str] = []
    if isinstance(blocks, dict):
        for locale_blocks in blocks.values():
            if isinstance(locale_blocks, list):
                for b in locale_blocks:
                    if isinstance(b, dict) and b.get("text"):
                        parts.append(str(b["text"]))
    elif isinstance(blocks, list):
        for b in blocks:
            if isinstance(b, dict) and b.get("text"):
                parts.append(str(b["text"]))
    return "\n".join(parts)


def extract_refs(text: str) -> list[dict[str, str]]:
    found: list[dict[str, str]] = []
    seen: set[str] = set()
    for pat, fmt in PATTERNS:
        for m in pat.finditer(text or ""):
            groups = m.groups()
            # Skip incomplete appendix matches without code
            try:
                code = fmt.format(*[g for g in groups if g is not None])
            except IndexError:
                continue
            # Special-case appendix with optional point
            if "Прил" in fmt and len(groups) >= 2 and groups[1]:
                code = f"ПДД Прил.№{groups[0]} п.{groups[1]}"
            title = m.group(0).strip()
            title = re.sub(r"\s+", " ", title)
            if len(title) > 120:
                title = title[:117] + "…"
            key = code.lower()
            if key in seen:
                continue
            seen.add(key)
            found.append({"code": code, "title": title})
            if len(found) >= 5:
                return found
    return found


def main() -> int:
    data = json.loads(DATA.read_text(encoding="utf-8"))
    explanations = data.get("explanations") or []
    filled = 0
    for row in explanations:
        text = blocks_text(row.get("blocks"))
        refs = extract_refs(text)
        if refs:
            row["legal_refs"] = refs
            filled += 1
        else:
            row["legal_refs"] = []
    DATA.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"explanations={len(explanations)} with_refs={filled}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
