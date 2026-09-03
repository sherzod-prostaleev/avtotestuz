#!/usr/bin/env python3
"""Stage 1: shortlist 2-3 candidate twins per source for every one of our questions.

This is the ONLY programmatic/fuzzy step in the pipeline, and it decides
nothing — it just narrows what a Stage 2 reader has to look at. Uses
Uzbek-Latin text as the primary key (matches the lesson from the ptest
comparison: Russian-only comparison undercounts badly).

Reads:
  data/our_questions.json
  data/avtodrom_ref.json
  data/ptest_questions.json + data/ptest_tag_map.json
  data/osonprava_questions.json (optional — skipped with a warning if absent,
    since it depends on the browser-harvest task which may run separately)

Writes:
  data/stage1_candidates.json - {ext_id: {source: [{ref, sim, category}, ...]}}
"""
import json
import re
from difflib import SequenceMatcher
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
DATA = REPO / "scratch/category-audit/data"
TOP_N = 3
MIN_SIM = 0.55  # below this, not worth a reader's time


def norm(s):
    s = s.lower()
    s = re.sub(r"[^\wʻʼ'\s]", "", s, flags=re.UNICODE)
    return re.sub(r"\s+", " ", s).strip()


def top_matches(query_norm, pool):
    """pool: list of (norm_text, payload). Returns up to TOP_N (sim, payload) above MIN_SIM."""
    scored = []
    for text, payload in pool:
        sim = SequenceMatcher(None, query_norm, text).ratio()
        if sim >= MIN_SIM:
            scored.append((sim, payload))
    scored.sort(key=lambda x: -x[0])
    return scored[:TOP_N]


def main():
    ours = json.loads((DATA / "our_questions.json").read_text())

    avtodrom = json.loads((DATA / "avtodrom_ref.json").read_text())
    avtodrom_pool = [
        (norm(r["question"]["uz-Latn"]), {"ref": r["avtodrom_id"], "category": r["category"]})
        for r in avtodrom
    ]

    ptest = json.loads((DATA / "ptest_questions.json").read_text())
    tag_map = json.loads((DATA / "ptest_tag_map.json").read_text())["tag_id_to_code"]
    ptest_pool = []
    for q in ptest:
        topic_codes = [tag_map[t] for t in q.get("tagIds", []) if t in tag_map]
        ptest_pool.append((
            norm(q["question"]["uz"]),
            {"ref": q["id"], "category": topic_codes[0] if topic_codes else None},
        ))

    osonprava_path = DATA / "osonprava_questions.json"
    osonprava_pool = []
    if osonprava_path.exists():
        osonprava = json.loads(osonprava_path.read_text())
        osonprava_pool = [
            (norm(r["question"]), {"ref": r["id"], "category": r["category"]})
            for r in osonprava
        ]
        print(f"osonprava: {len(osonprava_pool)} candidates loaded")
    else:
        print(f"WARNING: {osonprava_path} not found — osonprava candidates will be empty "
              f"for every question. Run the osonprava harvest task first if you need them.")

    out = {}
    for i, q in enumerate(ours):
        qn = norm(q["texts"]["uz-Latn"])
        out[q["ext_id"]] = {
            "current_category": q["category"],
            "avtodrom": [{"ref": p["ref"], "category": p["category"], "sim": round(s, 3)}
                         for s, p in top_matches(qn, avtodrom_pool)],
            "ptest": [{"ref": p["ref"], "category": p["category"], "sim": round(s, 3)}
                      for s, p in top_matches(qn, ptest_pool)],
            "osonprava": [{"ref": p["ref"], "category": p["category"], "sim": round(s, 3)}
                          for s, p in top_matches(qn, osonprava_pool)],
        }
        if (i + 1) % 200 == 0:
            print(f"  {i + 1}/{len(ours)}")

    (DATA / "stage1_candidates.json").write_text(json.dumps(out, ensure_ascii=False, indent=1))

    no_signal = sum(1 for v in out.values() if not (v["avtodrom"] or v["ptest"] or v["osonprava"]))
    print(f"wrote {len(out)} rows -> stage1_candidates.json ({no_signal} with zero candidates anywhere)")


if __name__ == "__main__":
    main()
