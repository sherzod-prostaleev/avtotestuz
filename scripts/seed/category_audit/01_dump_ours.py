#!/usr/bin/env python3
"""Dump our current question corpus to a flat audit-friendly JSON.

Reads backend/seed/avtoimtihon/data.json (categories + questions) and writes
scratch/category-audit/data/our_questions.json: one row per question with its
ext_id, current category code, and all three locale texts (question + answers).
"""
import json
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
SRC = REPO / "backend/seed/avtoimtihon/data.json"
OUT = REPO / "scratch/category-audit/data/our_questions.json"


def main():
    data = json.loads(SRC.read_text())
    categories = {c["code"]: c["sort"] for c in data["categories"]}

    rows = []
    for q in data["questions"]:
        if q["category"] not in categories:
            raise SystemExit(f"unknown category {q['category']!r} on {q['ext_id']}")
        rows.append({
            "ext_id": q["ext_id"],
            "category": q["category"],
            "category_sort": categories[q["category"]],
            "texts": q["texts"],
            "answers": [
                {"correct": a["correct"], "texts": a["texts"]}
                for a in q["answers"]
            ],
            "image": q.get("image"),
        })

    rows.sort(key=lambda r: int(r["ext_id"].split("-")[1]))
    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(json.dumps(rows, ensure_ascii=False, indent=1))
    print(f"wrote {len(rows)} questions -> {OUT}")

    by_cat = {}
    for r in rows:
        by_cat[r["category"]] = by_cat.get(r["category"], 0) + 1
    assert sum(by_cat.values()) == len(rows)
    print(f"{len(by_cat)} categories represented")


if __name__ == "__main__":
    main()
