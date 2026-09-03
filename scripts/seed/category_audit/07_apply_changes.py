#!/usr/bin/env python3
"""Stage 4: apply accepted category changes to the real seed files.

Reads data/accepted_changes.json — the Stage 4 consolidation output, a list
of {"ext_id": ..., "from": <old code>, "to": <new code>} for every change that
survived Stage 2 + Stage 3 agreement (or main-session escalation). Every row
must be a genuine change (from != to); the consolidation step is responsible
for filtering out confirmed-unchanged rows before this script ever sees them.

Updates, in place, preserving each file's exact existing JSON formatting
(data.json: indent=1 with a trailing newline; assignments.json: indent=2, no
trailing newline — see the 2026-09-03 ptest-question-add commit for why this
matters: json.dump's default indent reformats the *entire* file into a
90000-line diff if it doesn't match).

Does NOT commit or deploy. Prints a summary; leaves the working tree diff for
a human to review with `git diff --stat` / `git diff`.
"""
import json
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
DATA = REPO / "scratch/category-audit/data"
SEED = REPO / "backend/seed/avtoimtihon"


def main():
    changes = json.loads((DATA / "accepted_changes.json").read_text())
    by_ext_id = {}
    for c in changes:
        if c["from"] == c["to"]:
            raise SystemExit(f"{c['ext_id']}: from == to == {c['to']!r} — this "
                              f"should have been filtered out before this script runs")
        by_ext_id[c["ext_id"]] = c

    data = json.loads((SEED / "data.json").read_text())
    categories = {cat["code"] for cat in data["categories"]}
    for c in changes:
        if c["to"] not in categories:
            raise SystemExit(f"{c['ext_id']}: target category {c['to']!r} is not one of our 42")

    applied = []
    for q in data["questions"]:
        c = by_ext_id.get(q["ext_id"])
        if c is None:
            continue
        if q["category"] != c["from"]:
            raise SystemExit(
                f"{q['ext_id']}: expected current category {c['from']!r}, "
                f"found {q['category']!r} — the audit data is stale, re-run "
                f"Stage 1 onward before applying"
            )
        q["category"] = c["to"]
        applied.append(q["ext_id"])

    missing = set(by_ext_id) - set(applied)
    if missing:
        raise SystemExit(f"{len(missing)} ext_ids in accepted_changes.json don't "
                          f"exist in data.json: {sorted(missing)}")

    with open(SEED / "data.json", "w") as f:
        json.dump(data, f, ensure_ascii=False, indent=1)
        f.write("\n")

    assignments = json.loads((SEED / "assignments.json").read_text())
    for ext_id in applied:
        assignments[ext_id] = by_ext_id[ext_id]["to"]
    with open(SEED / "assignments.json", "w") as f:
        json.dump(assignments, f, ensure_ascii=False, indent=2)

    print(f"applied {len(applied)} category changes to data.json + assignments.json")
    by_target = {}
    for c in changes:
        by_target.setdefault(c["to"], []).append(c["ext_id"])
    for code, ids in sorted(by_target.items()):
        print(f"  -> {code}: +{len(ids)} ({', '.join(ids[:5])}{'...' if len(ids) > 5 else ''})")


if __name__ == "__main__":
    main()
