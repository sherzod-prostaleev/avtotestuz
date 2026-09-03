#!/usr/bin/env python3
"""Merge yolharakatiqoidalari.uz's three locale files into one audit-ready reference.

Each of questions_uzl.json / questions_uzk.json / questions_ru.json has the
same `id` in the same order (verified 2026-09-03: 1264/1264/1264, ids 1..1264
aligned across all three). This script joins them by id and attaches the
`topic` -> our category code via category_map.json.
"""
import json
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
AVTODROM_DIR = Path("/home/sher/Рабочий стол/avtodrom/data")
MAP = REPO / "scratch/category-audit/data/category_map.json"
OUT = REPO / "scratch/category-audit/data/avtodrom_ref.json"


def main():
    uzl = json.loads((AVTODROM_DIR / "questions_uzl.json").read_text())
    uzk = json.loads((AVTODROM_DIR / "questions_uzk.json").read_text())
    ru = json.loads((AVTODROM_DIR / "questions_ru.json").read_text())
    assert len(uzl) == len(uzk) == len(ru), (len(uzl), len(uzk), len(ru))

    by_id_uzk = {q["id"]: q for q in uzk}
    by_id_ru = {q["id"]: q for q in ru}

    topic_to_code = json.loads(MAP.read_text())["avtodrom_topic_to_code"]

    rows = []
    locale_mismatches = []
    for q in uzl:
        qk = by_id_uzk[q["id"]]
        qr = by_id_ru[q["id"]]
        if not (len(q["answers"]) == len(qk["answers"]) == len(qr["answers"])):
            # Source data quality glitch (their export, not ours) — keep the
            # row (uz-Latn/uz-Cyrl are what the audit reads primarily) but
            # drop the mismatched ru answer list rather than mis-pairing it,
            # and record the id so a reader can sanity-check citations there.
            locale_mismatches.append(q["id"])
            ru_answers = None
        else:
            ru_answers = [qr["answers"][i] for i in range(len(q["answers"]))]

        code = topic_to_code[str(q["topic"])]
        rows.append({
            "avtodrom_id": q["id"],
            "category": code,
            "question": {"uz-Latn": q["question"], "uz-Cyrl": qk["question"], "ru": qr["question"]},
            "answers": [
                {
                    "uz-Latn": q["answers"][i],
                    "uz-Cyrl": qk["answers"][i],
                    "ru": ru_answers[i] if ru_answers else None,
                }
                for i in range(len(q["answers"]))
            ],
            "correct_answer_index": q["correct_answer"] - 1,  # source is 1-based
            "citation": {"uz-Latn": q["correct_ans_alls"], "ru": qr["correct_ans_alls"]},
        })

    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(json.dumps(rows, ensure_ascii=False, indent=1))
    no_citation = sum(1 for r in rows if not r["citation"]["uz-Latn"].strip())
    print(f"wrote {len(rows)} rows -> {OUT} ({no_citation} without a citation)")
    if locale_mismatches:
        print(f"{len(locale_mismatches)} ids have a ru answer-count mismatch "
              f"(ru answers dropped, uz-Latn/uz-Cyrl kept): {locale_mismatches}")


if __name__ == "__main__":
    main()
