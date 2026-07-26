#!/usr/bin/env python3
"""Strict rebuild of question↔sign links for practice-by-sign.

Rules (practice subject only — no distractors):
1. Explicit sign codes in question or correct-answer text (with sign context).
2. «Qaysi belgi…» grids: keep the subject sign via (a) ordered option list +
   correct choice, or (b) catalog-name match among previous candidates with a
   clear score margin. Otherwise clear (empty is better than a false link).
3. Scene questions: at most one main sign (+ 7.x plates on that post).
"""

from __future__ import annotations

import json
import re
import sys
from difflib import SequenceMatcher
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
AVTO = ROOT / "backend/seed/avtoimtihon/data.json"
SIGNS = ROOT / "backend/seed/signs/data.json"
OUT = ROOT / "backend/seed/avtoimtihon/question_signs.json"
ORDERED = Path("/tmp/strict-which-ordered.json")

CODE_RE = re.compile(r"(?<![\d.])(\d{1,2}\.\d{1,2}(?:\.\d{1,2})?)(?![\d.])")
WHICH_RE = re.compile(
    r"qaysi belgi|belgilardan qaysi|belgilaridan qaysi|qaysi belgilari|"
    r"belgilarning qaysi|belgilardan qay |belgilaridan qay |"
    r"ogohlantiruvchi belgilarni|ushbu belgilardan qaysi|ushbu belgilaridan qaysi|"
    r"ko'rsatilgan belgilardan qaysi|ko'rsatilgan belgilaridan qaysi|"
    r"ko'rsatilgan qaysi belgi|ko'rsatilgan yo'l belgilaridan qaysi|"
    r"ko'rsatilgan yo'l belgilardan qaysi|"
    r"quyidagi belgilardan qaysi|quyidagi belgilaridan qaysi|bu belgilardan qaysi|"
    r"belgilardan qay biri|belgilaridan qay biri|mazkur yo'l belgisi nimani|bu belgi nimani|"
    r"ushbu belgi qanday|ushbu yo'l belgisi",
    re.I,
)
LETTER_TABLE = {
    "a": 1, "а": 1,
    "b": 2, "б": 2,
    "c": 3, "в": 3, "v": 3,
    "d": 4, "г": 4, "g": 4,
    "e": 5, "д": 5,
}


def norm(s: str) -> str:
    s = (s or "").lower().replace("ʻ", "'").replace("`", "'").replace("’", "'")
    s = re.sub(r"[«»\"'.,:;!?()]+", " ", s)
    return re.sub(r"\s+", " ", s).strip()


def load_catalog() -> tuple[set[str], dict[str, dict[str, str]]]:
    rows = json.loads(SIGNS.read_text(encoding="utf-8"))["signs"]
    codes = set()
    meta: dict[str, dict[str, str]] = {}
    for s in rows:
        codes.add(s["code"])
        names = s.get("names") or {}
        meta[s["code"]] = {
            "uz": names.get("uz-Latn") or "",
            "ru": names.get("ru") or "",
            "cyrl": names.get("uz-Cyrl") or "",
        }
    return codes, meta


def correct_answer(q: dict) -> dict | None:
    for a in q.get("answers") or []:
        if a.get("correct"):
            return a
    return None


def codes_in_text(text: str, catalog: set[str]) -> list[str]:
    out: list[str] = []
    for m in CODE_RE.findall(text or ""):
        if m in catalog and m not in out:
            out.append(m)
    return out


def meaning_clause(uz: str) -> str:
    for p in (
        r"qaysi belgi\s+(.+?)(?:\?|$)",
        r"belgilardan qaysi(?:\s+biri)?\s+(.+?)(?:\?|$)",
        r"belgilardan qay biri\s+(.+?)(?:\?|$)",
        r"qaysi belgilari?\s+(.+?)(?:\?|$)",
        r"ko'rsatilgan qaysi belgi\s+(.+?)(?:\?|$)",
        r"ushbu belgilardan qaysi(?:\s+biri)?\s+(.+?)(?:\?|$)",
        r"bu belgilardan qaysi(?:\s+biri)?\s+(.+?)(?:\?|$)",
        r"quyidagi belgilardan qaysi(?:\s+biri)?\s+(.+?)(?:\?|$)",
        r"mazkur yo'l belgisi\s+(.+?)(?:\?|$)",
        r"bu belgi\s+(.+?)(?:\?|$)",
        r"ushbu belgi\s+(.+?)(?:\?|$)",
        r"ushbu yo'l belgisi\s+(.+?)(?:\?|$)",
    ):
        m = re.search(p, uz or "", re.I)
        if m:
            return m.group(1).strip()
    return ""


def name_score(clause: str, code: str, meta: dict[str, dict[str, str]]) -> float:
    c = norm(clause)
    if not c:
        return 0.0
    info = meta[code]
    best = 0.0
    forbid = bool(re.search(r"taqiql|запрещ", " ".join(info.values()), re.I))
    allow = bool(re.search(r"ruxsat|разреш", c, re.I))
    for n in info.values():
        nn = norm(n)
        if not nn:
            continue
        if nn in c or c in nn:
            best = max(best, 0.96)
        ct, nt = set(c.split()), set(nn.split())
        if ct and nt:
            best = max(best, len(ct & nt) / len(ct | nt))
        best = max(best, SequenceMatcher(None, c, nn).ratio())
    if allow and forbid:
        best -= 0.25
    return best


def parse_positions(correct_text: str, n_options: int) -> list[int] | None:
    t = (correct_text or "").strip()
    if not t or n_options <= 0:
        return None
    if re.fullmatch(r"\d{1,2}", t):
        pos = int(t)
        return [pos] if 1 <= pos <= n_options else None

    if re.search(r"hammasi|barcha javoblar to'g'ri|har ikkisida|har ikkisi", t, re.I):
        # Group questions — do not map to every distractor via positions.
        return None

    # Prefer letter labels («A», «Б») over bare digits — answer.position can disagree.
    # Only letters inside quotes/guillemets (avoid matching the "a" in "Faqat").
    labeled = re.findall(r"[«\"'„]\s*([A-Da-dАБВГабвг])\s*[»\"']", t)
    if not labeled:
        labeled = re.findall(r"\b([A-D])\b", t)
    if labeled:
        pos: list[int] = []
        for ch in labeled:
            i = LETTER_TABLE.get(ch.lower())
            if i and i <= n_options and i not in pos:
                pos.append(i)
        if pos:
            return pos

    nums = [int(x) for x in re.findall(r"\d{1,2}", t)]
    low = norm(t)
    word_map = {
        "birinchi": 1, "ikkinchi": 2, "uchinchi": 3,
        "tortinchi": 4, "to'rtinchi": 4, "beshinchi": 5,
    }
    for w, i in word_map.items():
        if w in low:
            nums.append(i)
    pos = []
    for n in nums:
        if 1 <= n <= n_options and n not in pos:
            pos.append(n)
    if re.search(r"birinchi belgi", t, re.I):
        return [1]
    return pos or None

def select_ordered(ordered: list[str], positions: list[int]) -> list[str]:
    out: list[str] = []
    for p in positions:
        if 1 <= p <= len(ordered):
            code = ordered[p - 1]
            if code and code not in out:
                out.append(code)
    return out


def pick_by_name(
    clause: str,
    candidates: list[str],
    meta: dict[str, dict[str, str]],
) -> list[str]:
    if not clause or not candidates:
        return []
    scored = sorted(
        ((name_score(clause, c, meta), c) for c in candidates),
        reverse=True,
    )
    if not scored:
        return []
    top, code = scored[0]
    second = scored[1][0] if len(scored) > 1 else 0.0
    if top >= 0.55 and (top - second) >= 0.06:
        return [code]
    if top >= 0.72:
        return [code]
    return []


def main() -> int:
    catalog, meta = load_catalog()
    questions = json.loads(AVTO.read_text(encoding="utf-8"))["questions"]
    old = json.loads(OUT.read_text(encoding="utf-8")) if OUT.exists() else {}
    ordered_map: dict[str, list[str]] = {}
    if ORDERED.exists():
        raw = json.loads(ORDERED.read_text(encoding="utf-8"))
        for k, v in raw.items():
            ordered_map[k] = [c for c in v if c in catalog]

    # Merge agent part files (skip noisy *-auto* dumps). Prefer lists whose
    # length matches the question's answer count.
    q_by_id = {q["ext_id"]: q for q in questions}
    for part in sorted(Path("/tmp").glob("strict-which-ordered-part*.json")):
        if "auto" in part.name:
            continue
        raw = json.loads(part.read_text(encoding="utf-8"))
        for k, v in raw.items():
            cleaned: list[str] = []
            for c in v:
                if c in catalog and c not in cleaned:
                    cleaned.append(c)
            n_opt = len((q_by_id.get(k) or {}).get("answers") or [])
            prev = ordered_map.get(k, [])
            prev_match = bool(n_opt and len(prev) == n_opt)
            new_match = bool(n_opt and len(cleaned) == n_opt)
            if new_match and not prev_match:
                ordered_map[k] = cleaned
            elif new_match and prev_match and len(cleaned) >= len(prev):
                ordered_map[k] = cleaned
            elif not prev and cleaned:
                ordered_map[k] = cleaned
    rebuilt: dict[str, list[str]] = {}
    stats = {
        "text": 0,
        "which_ordered": 0,
        "which_singleton": 0,
        "which_name": 0,
        "which_cleared": 0,
        "scene": 0,
        "scene_cleared": 0,
    }

    for q in questions:
        ext = q["ext_id"]
        uz = (q.get("texts") or {}).get("uz-Latn") or ""
        ca = correct_answer(q)
        ca_uz = ((ca.get("texts") or {}).get("uz-Latn") if ca else "") or ""
        ca_blob = " ".join((ca.get("texts") or {}).values()) if ca else ""
        all_ans = " ".join(
            " ".join((a.get("texts") or {}).values()) for a in (q.get("answers") or [])
        )
        text_blob = uz + "\n" + ca_blob

        text_codes = codes_in_text(text_blob, catalog)
        # Pavement-marking questions cite codes like «1.1» / «1.11» for lines,
        # not warning sign 1.1 (railway). Never promote those to practice-by-sign.
        if re.search(
            r"yo'?l chizig|chiziq nimani|uzuq-uzuq chiziq|1\.1 yoki 1\.11|1\.11 chizi",
            text_blob + "\n" + all_ans,
            re.I,
        ) and not re.search(r"\bbelgi\b|знак", uz, re.I):
            text_codes = []
        if not q.get("image"):
            # answers may cite 5.15 / 6.11 etc.
            text_codes = codes_in_text(text_blob + "\n" + all_ans, catalog)
            if text_codes and not re.search(
                r"belgi|знак|avtomagistral|piyoda|aholi punkt|to'xtash|STOP|tramvay",
                text_blob + all_ans,
                re.I,
            ):
                text_codes = []
            if re.search(
                r"yo'?l chizig|chiziq nimani|uzuq-uzuq chiziq|1\.1 yoki 1\.11",
                text_blob + "\n" + all_ans,
                re.I,
            ) and not re.search(r"\bbelgi\b|знак", uz, re.I):
                text_codes = []

        chosen: list[str] = list(text_codes)
        prev = [c for c in old.get(ext, []) if c in catalog]
        is_which = bool(WHICH_RE.search(uz))
        ordered = ordered_map.get(ext, [])

        if is_which:
            n_opt = len(q.get("answers") or []) or len(ordered)
            positions = parse_positions(ca_uz, n_opt)
            picked: list[str] = []
            if ordered and positions:
                picked = select_ordered(ordered, positions)
                if picked:
                    stats["which_ordered"] += 1
            # Vision stub: single subject sign (meaning-text answers, not A/B/1/2).
            if not picked and len(ordered) == 1:
                picked = list(ordered)
                stats["which_singleton"] += 1
            if not picked:
                clause = meaning_clause(uz)
                picked = pick_by_name(clause, prev or ordered, meta)
                if picked:
                    stats["which_name"] += 1
            if not picked and text_codes:
                picked = list(text_codes)
            if picked:
                chosen = sorted(set(chosen) | set(picked))
            else:
                # Explicitly drop distractor piles
                if prev and not chosen:
                    stats["which_cleared"] += 1
                chosen = sorted(set(chosen))
        elif q.get("image"):
            mains = [c for c in prev if not c.startswith("7.")]
            plates = [c for c in prev if c.startswith("7.")]
            if len(mains) <= 1:
                chosen = sorted(set(chosen) | set(mains) | set(plates))
                if mains or plates or chosen:
                    stats["scene"] += 1
            elif ordered:
                om = [c for c in ordered if not c.startswith("7.")]
                op = [c for c in ordered if c.startswith("7.")]
                keep = (om[:1] if om else []) + op
                chosen = sorted(set(chosen) | set(keep))
                stats["scene"] += 1
            else:
                # Ambiguous multi-sign scene: pick one primary by question keywords.
                clause = meaning_clause(uz) or uz
                picked = pick_by_name(clause, mains, meta)
                if not picked:
                    # Prefer mandatory/priority/prohibitory over info/service noise
                    def rank(code: str) -> tuple:
                        g = code.split(".")[0]
                        pri = {"4": 0, "2": 1, "3": 2, "1": 3, "5": 4, "6": 5}.get(g, 9)
                        return (pri, code)
                    picked = [sorted(mains, key=rank)[0]] if mains else []
                keep_plates = [c for c in plates if True]  # keep plates with chosen primary
                # Only keep plates if we had exactly one main originally adjacent — keep all plates when one main chosen
                chosen = sorted(set(chosen) | set(picked) | (set(plates) if len(picked) == 1 else set()))
                if picked:
                    stats["scene"] += 1
                elif not chosen:
                    stats["scene_cleared"] += 1
        else:
            if text_codes:
                stats["text"] += 1

        if chosen:
            rebuilt[ext] = chosen

    OUT.write_text(json.dumps(rebuilt, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    nonempty = len(rebuilt)
    rows = sum(len(v) for v in rebuilt.values())
    signs = len({c for v in rebuilt.values() for c in v})
    print(
        f"wrote {OUT}: nonempty={nonempty} rows={rows} unique_signs={signs} stats={stats}",
        file=sys.stderr,
    )

    # Sanity: known gold
    gold = {
        "avtoimtihon-23": ["2.6"],
        "avtoimtihon-1": ["3.27", "7.18"],
        "avtoimtihon-31": ["1.20"],
        "avtoimtihon-532": ["5.5"],
    }
    for ext, want in gold.items():
        got = rebuilt.get(ext, [])
        if got != want:
            print(f"GOLD WARN {ext}: got {got} want {want}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
