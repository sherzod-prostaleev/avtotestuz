# 42-Topic Category Re-Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Re-verify the `category` field of all 1273 questions in `backend/seed/avtoimtihon/data.json` by reading each one against three reference sources, correcting every mistake found, without touching anything else.

**Architecture:** A one-way data pipeline: committed scripts in `scripts/seed/category_audit/` read/write working data in `scratch/category-audit/data/` (gitignored — the repo's existing `scratch/` rule covers it). Harvest four datasets (ours + 3 references) → cheap fuzzy shortlist per question → careful reading pass by parallel subagents → independent second reading pass on anything that moved → consolidate into one report and apply the accepted changes to the two real seed files. Nothing is committed or deployed by this plan — the last task stops at a reviewable `git diff`.

**Tech Stack:** Python 3 (stdlib only — `json`, `difflib`, `subprocess`, no dependencies to install) for every scripted step; `curl` for ptest.uz; Chrome browser automation (`mcp__claude-in-chrome__*`) for osonprava.uz, which has no bulk API; Docker (`golang:1.27`) for the final `importer.Validate` check, per the `go-toolchain-docker-orqali` memory (no local `go` binary on this machine).

## Global Constraints

- Ground truth is the real YHQ law, not the reference sites — a citation or a clear reading of the rule beats 2-of-3 site agreement (spec §"Decision rule, per question").
- Matching between corpora is by reading Uzbek-Latin text (Russian as a second opinion), never by image, never by trusting a bare fuzzy-similarity score as the final word (spec §"Matching method").
- Scope is category correction only: no new questions imported, no taxonomy/schema change, no code change — two JSON seed files only (spec §"What changes, and what deliberately does not").
- Every decision (kept or changed) needs one stated line of reasoning naming what the question actually tests — "majority of sources agreed" is not sufficient reasoning on its own (spec §"Decision rule, per question", point 5).
- Nothing is committed or deployed by this plan. The final task ends at an unstaged `git diff` for the user to review (spec §"Pipeline", Stage 4, and §"Risk").
- Scripts are committed tooling and live in `scripts/seed/category_audit/` (following the existing `scripts/seed/` convention — e.g. `extract_legal_refs.py`), **not** under `scratch/`, which is blanket-gitignored (a bare `git add scratch/...` silently no-ops there — confirmed while writing this plan). Only harvested/intermediate *data* goes in `scratch/category-audit/data/`.

---

## File structure

```
scripts/seed/category_audit/
  01_dump_ours.py              # our 1273 questions -> flat JSON
  02_build_category_map.py     # canonical code <-> avtodrom topic# <-> osonprava name map
  03_build_avtodrom_ref.py     # merge yolharakatiqoidalari.uz's 3 locale files
  04_harvest_ptest.py          # ptest.uz open API harvest (questions + tag map)
  05_shortlist_candidates.py   # Stage 1: fuzzy top-3 candidates per source
  06_merge_osonprava.py        # merge browser-harvested osonprava captures
  07_apply_changes.py          # Stage 4: write accepted changes into the seed files

scratch/category-audit/data/   # gitignored — never commit anything from here
  our_questions.json           # Task 1 output
  category_map.json            # Task 2 output
  avtodrom_ref.json            # Task 3 output
  ptest_questions.json         # Task 4 output
  ptest_tag_map.json           # Task 4 output
  osonprava_raw/<code>.json    # Task 5 output, one file per topic
  osonprava_questions.json     # Task 5 output (merged)
  stage1_candidates.json       # Task 6 output
  stage2_audit.json            # Task 7 output (merged from 7 subagent batches)
  stage3_reaudit.json          # Task 8 output
  escalations.json             # Task 9 output
  accepted_changes.json        # Task 9 output (Task 10's input)
  report.md                    # Task 10 output — what the user reviews
```

All four scripts in Tasks 1-4 and the shortlist script in Task 6 were written
and run once already during spec/plan development on 2026-09-03, including
fixing the `scratch/`-vs-`scripts/seed/` mistake above; their output files
exist right now in `scratch/category-audit/data/`. **Re-run them anyway as
part of executing this plan** — the *data* directory is gitignored, so a
fresh worktree or a new session will not have it, and the harvested data
(ptest's live corpus especially) can have moved on since. The *scripts*
directory, once Task 1-4/6's commit steps run, will already exist and not
need rewriting — just execute it.

---

## Task 1: Dump our current question corpus

**Files:**
- Create: `scripts/seed/category_audit/01_dump_ours.py`
- Reads: `backend/seed/avtoimtihon/data.json`
- Produces: `scratch/category-audit/data/our_questions.json`

**Interfaces:**
- Produces: a JSON array, each element `{ext_id, category, category_sort, texts: {ru, uz-Cyrl, uz-Latn}, answers: [{correct, texts}], image}`. Every later task that reads "our" questions reads this file, not `data.json` directly.

- [ ] **Step 1: Write the script**

```python
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
```

- [ ] **Step 2: Run it and verify the count**

Run: `python3 scripts/seed/category_audit/01_dump_ours.py`
Expected: `wrote 1273 questions -> .../our_questions.json` and `42 categories represented`. If the count isn't 1273, `git log --oneline -- backend/seed/avtoimtihon/data.json` to see whether more content landed since 2026-09-03 and treat the new total as correct — this script has no hardcoded expectation, the number just needs to be sane (matches `wc -l < backend/seed/avtoimtihon/data.json` order of magnitude, no crash).

- [ ] **Step 3: Commit the script (not the data — `scratch/` is gitignored)**

```bash
git add scripts/seed/category_audit/01_dump_ours.py
git commit -m "$(cat <<'EOF'
chore(audit): add category-audit Task 1 script (dump our corpus)

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Build the canonical category map

**Files:**
- Create: `scripts/seed/category_audit/02_build_category_map.py`
- Reads: `/home/sher/Рабочий стол/avtodrom/data/questions_uzl.json` (the owner's other project — local files, not a URL)
- Produces: `scratch/category-audit/data/category_map.json`

**Interfaces:**
- Produces: `{avtodrom_topic_to_code, code_to_avtodrom_topic, osonprava_name_to_code, code_to_osonprava_name, no_osonprava_match, osonprava_only_topics}`. Task 3 uses `avtodrom_topic_to_code`. Task 5 (osonprava harvest) uses `code_to_osonprava_name`. Task 6 uses `code_to_osonprava_name`'s keys as the set of topics to expect a raw capture file for.

- [ ] **Step 1: Write the script**

```python
#!/usr/bin/env python3
"""Build the canonical category map: our 42 codes <-> avtodrom topic # <-> osonprava topic name.

avtodrom (yolharakatiqoidalari.uz local source) topics are 1..42 in the exact
same order as our category.sort — verified 2026-09-03 by comparing each
topic's question count against the live site's /main/mavzulashtirilgan_testlar
list, in order. That verification is re-asserted here against the local JSON
so a future change to the avtodrom data trips a loud error instead of silently
mis-mapping.

osonprava.uz topics do NOT share our order (they're grouped into 9 sections on
their /topics page) and are missing our `officials_duties` while carrying an
extra "Taniqli belgilar" topic we don't have — so the osonprava side is a name
lookup table, hand-built from the /topics page text captured 2026-09-03, with
two rows deliberately absent (see NO_OSONPRAVA_MATCH / OSONPRAVA_ONLY below).

ptest.uz maps by tag name too (its tag catalogue at group_id
f8ef030a-71ff-4b3e-919d-6943186ba898), but that lookup happens in the ptest
harvesting script (04_harvest_ptest.py) since it needs the live tag IDs, not
here.
"""
import json
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
AVTODROM_UZL = Path("/home/sher/Рабочий стол/avtodrom/data/questions_uzl.json")
OUT = REPO / "scratch/category-audit/data/category_map.json"

# sort (1-42) -> our category code, exactly as in backend/seed/avtoimtihon/data.json
CATEGORY_ORDER = [
    "general_rules", "driver_duties", "pedestrian_duties", "special_vehicle_priority",
    "signs_warning", "signs_priority", "signs_prohibitory", "signs_mandatory",
    "signs_information", "signs_service", "signs_additional",
    "markings_horizontal", "markings_vertical",
    "traffic_lights", "traffic_controller", "warning_hazard_signals",
    "starting_manoeuvring", "lane_position", "speed_limits", "overtaking",
    "stopping_and_parking",
    "intersections_general", "intersections_regulated", "intersections_main_straight",
    "intersections_equal", "intersections_main_turns",
    "pedestrian_crossings_stops", "railway_crossings", "motorways",
    "residential_zones", "slopes", "public_transport_priority", "lighting_devices",
    "towing", "driver_training", "passenger_carriage", "cargo_carriage",
    "cyclists_mopeds_animals", "officials_duties", "vehicle_defects",
    "safety_basics", "first_aid",
]
assert len(CATEGORY_ORDER) == 42

# Per-topic question counts read from https://www.yolharakatiqoidalari.uz/main/mavzulashtirilgan_testlar
# on 2026-09-03, in the same top-to-bottom order as CATEGORY_ORDER. This is the
# order-alignment proof: if the local JSON's per-topic counts (computed below)
# don't match this list positionally, something has drifted and the script aborts.
AVTODROM_LIVE_COUNTS_2026_09_03 = [
    50, 13, 20, 11, 36, 29, 25, 56, 47, 35, 35, 74, 14, 21, 19, 48, 30, 15, 18,
    14, 9, 4, 20, 22, 30, 6, 18, 17, 15, 15, 37, 19, 63, 34, 66, 1, 41, 74, 5,
    49, 75, 34,
]
assert len(AVTODROM_LIVE_COUNTS_2026_09_03) == 42

# osonprava.uz topic name (uz-Latn, as rendered on /topics 2026-09-03) -> our code.
# Order here is irrelevant (name lookup, not positional). officials_duties has
# no counterpart; osonprava's "Taniqli belgilar" has no counterpart on our side.
OSONPRAVA_NAME_TO_CODE = {
    "Ogohlantiruvchi belgilar": "signs_warning",
    "Imtiyoz belgilari": "signs_priority",
    "Taqiqlovchi belgilar": "signs_prohibitory",
    "Buyuruvchi belgilar": "signs_mandatory",
    "Axborot-koʻrsatgich belgilari": "signs_information",
    "Servis belgilari": "signs_service",
    "Qoʻshimcha axborot belgilari": "signs_additional",
    "Chorrahalarda harakatlanish": "intersections_general",
    "Tartibga solingan chorrahalar": "intersections_regulated",
    "Tartibga solinmagan teng ahamiyatli chorrahalar": "intersections_equal",
    "Tartibga solinmagan chorrahalar (asosiy yo'l to'g'ri yo'nalishda)": "intersections_main_straight",
    "Tartibga solinmagan chorrahalar (asosiy yo'l yo'nalishi o'zgarishi)": "intersections_main_turns",
    "Svetofor ishoralari": "traffic_lights",
    "Tartibga soluvchining ishoralari": "traffic_controller",
    "Ogohlantiruvchi va avariya ishoralari": "warning_hazard_signals",
    "Yotiq chiziqlar": "markings_horizontal",
    "Tik chiziqlar": "markings_vertical",
    "Harakatlanishni boshlash, manyovr qilish": "starting_manoeuvring",
    "Yo'lning qatnov qismida transport vositalarining joylashuvi": "lane_position",
    "Harakatlanish tezligi": "speed_limits",
    "Quvib o'tish": "overtaking",
    "To'xtash va to'xtab turish": "stopping_and_parking",
    "Piyodalar o'tish joylari va yo'nalishli transport vositalari bekatlari": "pedestrian_crossings_stops",
    "Temir yo'l kesishmalari orqali harakatlanish": "railway_crossings",
    "Avtomagistrallarda harakatlanish": "motorways",
    "Turar joy dahalarida harakatlanish": "residential_zones",
    "Tik balandlik va nishabliklarda harakatlanish": "slopes",
    "Yo'nalishli transport vositalarining imtiyozlari": "public_transport_priority",
    "Maxsus transport vositalarining imtiyozlari": "special_vehicle_priority",
    "Tashqi yoritish asboblaridan foydalanish": "lighting_devices",
    "Umumiy qoidalar": "general_rules",
    "Haydovchilarning umumiy vazifalari": "driver_duties",
    "Piyodalarning umumiy vazifalari": "pedestrian_duties",
    "Mexanik transport vositalarini shatakka olish": "towing",
    "Transport vositalarini boshqarishni o'rgatish": "driver_training",
    "Odam tashish": "passenger_carriage",
    "Yuk tashish": "cargo_carriage",
    "Velosiped, moped va aravalar harakatlanishiga hamda hayvonlarni haydab o'tishga doir qo'shimcha talablar": "cyclists_mopeds_animals",
    "Birinchi tibbiy yordam": "first_aid",
    "Harakat xavfsizligi asoslari": "safety_basics",
    "Transport vositalaridan foydalanishni taqiqlovchi shartlar": "vehicle_defects",
}
NO_OSONPRAVA_MATCH = {"officials_duties"}   # our code with no osonprava topic
OSONPRAVA_ONLY = {"Taniqli belgilar"}       # their topic with no code on our side


def main():
    avtodrom = json.loads(AVTODROM_UZL.read_text())
    counts = {}
    for q in avtodrom:
        counts[q["topic"]] = counts.get(q["topic"], 0) + 1

    computed = [counts.get(i, 0) for i in range(1, 43)]
    if computed != AVTODROM_LIVE_COUNTS_2026_09_03:
        diffs = [
            (i + 1, CATEGORY_ORDER[i], computed[i], AVTODROM_LIVE_COUNTS_2026_09_03[i])
            for i in range(42) if computed[i] != AVTODROM_LIVE_COUNTS_2026_09_03[i]
        ]
        raise SystemExit(
            "avtodrom local data no longer matches the live-site order/count "
            f"snapshot used to prove positional alignment: {diffs}\n"
            "Re-verify topic order against https://www.yolharakatiqoidalari.uz/"
            "main/mavzulashtirilgan_testlar before trusting the topic-number map."
        )

    mapping = {
        "avtodrom_topic_to_code": {str(i + 1): CATEGORY_ORDER[i] for i in range(42)},
        "code_to_avtodrom_topic": {CATEGORY_ORDER[i]: i + 1 for i in range(42)},
        "osonprava_name_to_code": OSONPRAVA_NAME_TO_CODE,
        "code_to_osonprava_name": {v: k for k, v in OSONPRAVA_NAME_TO_CODE.items()},
        "no_osonprava_match": sorted(NO_OSONPRAVA_MATCH),
        "osonprava_only_topics": sorted(OSONPRAVA_ONLY),
    }

    assert set(OSONPRAVA_NAME_TO_CODE.values()) | NO_OSONPRAVA_MATCH == set(CATEGORY_ORDER), \
        "every one of our 42 codes must be either mapped to an osonprava name or listed in NO_OSONPRAVA_MATCH"

    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(json.dumps(mapping, ensure_ascii=False, indent=1))
    print(f"avtodrom order verified against {sum(AVTODROM_LIVE_COUNTS_2026_09_03)} live-site questions")
    print(f"osonprava name map: {len(OSONPRAVA_NAME_TO_CODE)} codes mapped, "
          f"{len(NO_OSONPRAVA_MATCH)} unmapped ({sorted(NO_OSONPRAVA_MATCH)})")
    print(f"wrote {OUT}")


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Run it and verify**

Run: `python3 scripts/seed/category_audit/02_build_category_map.py`
Expected:
```
avtodrom order verified against 1264 live-site questions
osonprava name map: 41 codes mapped, 1 unmapped (['officials_duties'])
wrote .../category_map.json
```
If it instead exits with "avtodrom local data no longer matches..." — the
owner's other project changed since 2026-09-03. Open
`https://www.yolharakatiqoidalari.uz/main/mavzulashtirilgan_testlar` (a real
account with an active session is not required for this list, only for the
per-question paywall), re-read the 42 counts top to bottom, and replace
`AVTODROM_LIVE_COUNTS_2026_09_03` with the new snapshot before continuing —
do not proceed on an unverified positional map.

- [ ] **Step 3: Commit**

```bash
git add scripts/seed/category_audit/02_build_category_map.py
git commit -m "$(cat <<'EOF'
chore(audit): add category-audit Task 2 script (category map)

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Build the yolharakatiqoidalari.uz reference

**Files:**
- Create: `scripts/seed/category_audit/03_build_avtodrom_ref.py`
- Reads: `/home/sher/Рабочий стол/avtodrom/data/questions_uz{l,k}.json`, `questions_ru.json`; `scratch/category-audit/data/category_map.json` (Task 2)
- Produces: `scratch/category-audit/data/avtodrom_ref.json`

**Interfaces:**
- Consumes: `category_map.json`'s `avtodrom_topic_to_code` (Task 2's output).
- Produces: a JSON array, each element `{avtodrom_id, category, question: {uz-Latn, uz-Cyrl, ru}, answers: [{uz-Latn, uz-Cyrl, ru}], correct_answer_index, citation: {uz-Latn, ru}}`. Tasks 6 and 7 read this for full text + citation.

- [ ] **Step 1: Write the script**

```python
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
```

- [ ] **Step 2: Run it and verify**

Run: `python3 scripts/seed/category_audit/03_build_avtodrom_ref.py`
Expected: `wrote 1264 rows -> .../avtodrom_ref.json (4 without a citation)` and
`1 ids have a ru answer-count mismatch ... [614]`. Both numbers came from a
real run 2026-09-03; if they differ, that just means the source data changed
— not an error, no action needed unless the run crashes.

- [ ] **Step 3: Commit**

```bash
git add scripts/seed/category_audit/03_build_avtodrom_ref.py
git commit -m "$(cat <<'EOF'
chore(audit): add category-audit Task 3 script (avtodrom reference)

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Harvest the ptest.uz reference

**Files:**
- Create: `scripts/seed/category_audit/04_harvest_ptest.py`
- Produces: `scratch/category-audit/data/ptest_questions.json`, `scratch/category-audit/data/ptest_tag_map.json`

**Interfaces:**
- Produces: `ptest_questions.json` = ptest's raw question objects (`id`, `question: {uz, cryl, ru}`, `answers: [{label: {uz, cryl, ru}, is_answer}]`, `tagIds`, ...) as returned by their API. `ptest_tag_map.json` = `{tags: [...], tag_id_to_code: {tag_id: our_code}}`. Tasks 6 and 7 read both.

- [ ] **Step 1: Write the script**

```python
#!/usr/bin/env python3
"""Harvest ptest.uz's full question corpus (with topic tags) via its open API.

Two outputs:
  data/ptest_questions.json - every unique question, via coupon-collector
    sampling of GET /api/v1/public/questions/random (20 random per call).
  data/ptest_tag_map.json   - the 42-topic tag catalogue, tag id -> our
    category code (positional — see PTEST_TOPIC_TAGS_LATIN_2026_09_03 below)
    plus the raw tag list for the mavzu group.

Method proven in this repo 2026-09-03 (see raqobatchi-savol-monitoring-bazasi
memory): curl with a real browser User-Agent (urllib gets 403'd by the WAF),
sequential with a small sleep, stop once a long streak of calls adds nothing
new.
"""
import json
import subprocess
import sys
import time
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
OUT_DIR = REPO / "scratch/category-audit/data"
UA = ("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/120.0 Safari/537.36")
RANDOM_URL = "https://ptest.uz/api/v1/public/questions/random"
TAGS_URL = "https://ptest.uz/api/v1/tags?limit=1000&page=1"
TOPIC_GROUP_ID = "f8ef030a-71ff-4b3e-919d-6943186ba898"


def curl_json(url):
    out = subprocess.run(["curl", "-sS", "-A", UA, url], capture_output=True, timeout=15)
    return json.loads(out.stdout)


# sort (1-42) -> our category code, exactly as in backend/seed/avtoimtihon/data.json
CATEGORY_ORDER = [
    "general_rules", "driver_duties", "pedestrian_duties", "special_vehicle_priority",
    "signs_warning", "signs_priority", "signs_prohibitory", "signs_mandatory",
    "signs_information", "signs_service", "signs_additional",
    "markings_horizontal", "markings_vertical",
    "traffic_lights", "traffic_controller", "warning_hazard_signals",
    "starting_manoeuvring", "lane_position", "speed_limits", "overtaking",
    "stopping_and_parking",
    "intersections_general", "intersections_regulated", "intersections_main_straight",
    "intersections_equal", "intersections_main_turns",
    "pedestrian_crossings_stops", "railway_crossings", "motorways",
    "residential_zones", "slopes", "public_transport_priority", "lighting_devices",
    "towing", "driver_training", "passenger_carriage", "cargo_carriage",
    "cyclists_mopeds_animals", "officials_duties", "vehicle_defects",
    "safety_basics", "first_aid",
]
assert len(CATEGORY_ORDER) == 42

# GET /api/v1/tags?limit=1000&page=1 returns the 42-topic group's tags in this
# exact order (verified 2026-09-03 — same positional order as our
# category.sort, same phenomenon as avtodrom's `topic` field). This snapshot
# (tag `latin` text, in API order) is the proof: if a re-fetch no longer lines
# up against it position-for-position, something changed upstream and the
# script must stop rather than silently mis-map.
PTEST_TOPIC_TAGS_LATIN_2026_09_03 = [
    "Umumiy qoidalar", "Haydovchilarning umumiy vazifalari",
    "Piyodalarning umumiy vazifalari", "Maxsus transport vositalarining imtiyozlari",
    "Ogohlantiruvchi belgilar", "Imtiyoz belgilari", "Taqiqlovchi belgilar",
    "Buyuruvchi belgilar", "Axborot-ko‘rsatgich belgilari", "Servis belgilari",
    "Qo‘shimcha axborot belgilari", "Yotiq chiziqlar", "Tik chiziqlar",
    "Svetofor ishoralari", "Tartibga soluvchining ishoralari",
    "Ogohlantiruvchi va avariya (xavf-xatar) ishoralari",
    "Harakatlanishni boshlash, manyovr qilish",
    "Yo‘lning qatnov qismida transport vositalarining joylashuvi",
    "Harakatlanish tezligi", "Quvib o‘tish", "To‘xtash va to‘xtab turish",
    "Chorrahalarda harakatlanish", "Tartibga solingan chorrahalar",
    "Tartibga solinmagan chorrahalar asosiy yo‘l yo‘nalishi to‘g‘riga",
    "Tartibga solinmagan chorrahalar teng ahamiyatli",
    "Tartibga solinmagan chorrahalar asosiy yo‘l yo‘nalishi o‘zgarishi",
    "Piyodalarning o‘tish joylari va yo‘nalishli transport vositalarining bekatlari",
    "Temir yo‘l kesishmalari orqali harakatlanish",
    "Avtomagistrallarda harakatlanish", "Turar joy dahalarida harakatlanish",
    "Tik balandlik va nishabliklarda harakatlanish",
    "Yo‘nalishli transport vositalarining imtiyozlari",
    "Tashqi yoritish asboblaridan foydalanish",
    "Mexanik transport vositalarini shatakka olish",
    "Transport vositalarini boshqarishni o‘rgatish", "Odam tashish", "Yuk tashish",
    "Velosiped, moped va aravalar harakatlanishiga, shuningdek, "
    "hayvonlarni haydab o‘tishga doir qo‘shimcha talablar",
    "Mansabdor shaxslarning va fuqarolarning yo‘l harakati xavfsizligini "
    "taminlash, transport vositalarini yo‘lga chiqarish, raqam va taniqli "
    "belgilarini o‘rnatish bo‘yicha majburiyatlari",
    "Transport vositalaridan foydalanishni taqiqlovchi shartlar",
    "Harakat xafsizligi asoslari", "Birinchi tibbiy yordam",
]
assert len(PTEST_TOPIC_TAGS_LATIN_2026_09_03) == 42


def harvest_questions(max_calls=900, stall_calls=150):
    seen = {}
    no_new_streak = 0
    call = 0
    while call < max_calls:
        call += 1
        try:
            data = curl_json(RANDOM_URL)
        except Exception as e:
            print(f"  err {e}", file=sys.stderr)
            time.sleep(0.5)
            continue
        new = 0
        for q in data:
            if q["id"] not in seen:
                seen[q["id"]] = q
                new += 1
        no_new_streak = 0 if new else no_new_streak + 1
        if call % 50 == 0:
            print(f"  call={call} unique={len(seen)} stall={no_new_streak}", file=sys.stderr)
        if no_new_streak >= stall_calls and call > 300:
            print(f"  stopping: {no_new_streak} calls with nothing new", file=sys.stderr)
            break
        time.sleep(0.03)
    return list(seen.values())


def build_tag_map():
    tags = curl_json(TAGS_URL)["data"]
    topic_tags = [t for t in tags if t["group_id"] == TOPIC_GROUP_ID]
    assert len(topic_tags) == 42, f"expected 42 topic tags, got {len(topic_tags)}"

    live_order = [t["latin"] for t in topic_tags]
    if live_order != PTEST_TOPIC_TAGS_LATIN_2026_09_03:
        diffs = [(i, a, b) for i, (a, b) in
                 enumerate(zip(live_order, PTEST_TOPIC_TAGS_LATIN_2026_09_03)) if a != b]
        raise SystemExit(
            "ptest's tag API no longer returns the 42 topic tags in the "
            f"snapshotted order — positional mapping is unsafe: {diffs}\n"
            "Re-derive the mapping (by reading each tag's name against our "
            "42 category names) before trusting tag_id_to_code."
        )

    tag_id_to_code = {topic_tags[i]["id"]: CATEGORY_ORDER[i] for i in range(42)}
    return {"tags": topic_tags, "tag_id_to_code": tag_id_to_code}


def main():
    OUT_DIR.mkdir(parents=True, exist_ok=True)

    print("fetching tag catalogue...", file=sys.stderr)
    tag_map = build_tag_map()
    (OUT_DIR / "ptest_tag_map.json").write_text(json.dumps(tag_map, ensure_ascii=False, indent=1))
    print(f"  {len(tag_map['tag_id_to_code'])}/42 topic tags mapped to our codes")

    print("harvesting questions (coupon collector, several minutes)...", file=sys.stderr)
    questions = harvest_questions()
    (OUT_DIR / "ptest_questions.json").write_text(json.dumps(questions, ensure_ascii=False, indent=1))
    print(f"wrote {len(questions)} questions -> {OUT_DIR / 'ptest_questions.json'}")


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Run it in the background and verify**

Run: `python3 scripts/seed/category_audit/04_harvest_ptest.py > scratch/category-audit/data/harvest_ptest.log 2>&1 &`
This takes 5-10 minutes (roughly 450-550 sequential API calls). Poll with
`tail -f scratch/category-audit/data/harvest_ptest.log` or a Monitor
until-loop on `pgrep -f 04_harvest_ptest`.
Expected tail: `42/42 topic tags mapped to our codes` then eventually
`stopping: 150 calls with nothing new` and `wrote <N> questions -> ...` where
N was 1309 on 2026-09-03 — treat a nearby number as fine (ptest's corpus
grows over time, see `raqobatchi-savol-monitoring-bazasi` memory), a wildly
smaller number (e.g. under 1000) as a sign the WAF started 403'ing again and
needs the User-Agent checked.

- [ ] **Step 3: Commit**

```bash
git add scripts/seed/category_audit/04_harvest_ptest.py
git commit -m "$(cat <<'EOF'
chore(audit): add category-audit Task 4 script (ptest harvest)

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Harvest the osonprava.uz reference (browser)

osonprava.uz has no bulk API reachable from outside an authenticated page
(confirmed 2026-09-03: calling `/api/app/sync/*` via injected `fetch` fails
with a CORS error even from within a logged-in tab). The only way to read
its content is clicking through the UI, but it needs no paywall and no
programmatic workaround — just volume.

**Files:**
- Create: `scripts/seed/category_audit/06_merge_osonprava.py` (this task's Step 3)
- Produces: `scratch/category-audit/data/osonprava_raw/<code>.json` (one per topic, this task's Step 2), `scratch/category-audit/data/osonprava_questions.json` (Step 4)

**Interfaces:**
- Consumes: `category_map.json`'s `code_to_osonprava_name` (Task 2).
- Produces: `osonprava_questions.json` = `[{id: "<code>:<index>", category, question, answers}]`. Task 6 reads this.

- [ ] **Step 1: Confirm the interaction pattern (already verified 2026-09-03, re-check if osonprava's frontend changed)**

Navigate to `https://app.osonprava.uz/topic-test/1` in a logged-in tab
(session already exists on this machine — Sherzod's account). The page shows
a horizontally-scrollable row of question-number buttons (1, 2, 3, ... up to
the topic's total) above the current question; clicking a number renders
that question's stem and its `F1`/`F2`/... answer options below, and
`get_page_text` on the tab returns that question's text cleanly. There is a
countdown timer (~51 minutes on load) — harmless at this pace, but don't let
one topic's harvest run past it.

- [ ] **Step 2: Harvest each topic, one subagent per group, in parallel**

Split the 41 osonprava-mapped topics (everything in `category_map.json`'s
`code_to_osonprava_name` — `officials_duties` has no osonprava topic, skip
it) across parallel subagents by total question count, similarly to Task 7's
split. A reasonable split (5 subagents, ~240 questions each) grouped by
`/topics` section so each subagent's mental context stays coherent:

- Subagent A — Yo'l belgilari (7 topics): signs_warning(38), signs_priority(23), signs_prohibitory(59), signs_mandatory(32), signs_information(60), signs_service(1), signs_additional(38)
- Subagent B — Chorrahalar + Ishoralar va chiziqlar (10 topics): intersections_general(14), intersections_regulated(20), intersections_equal(48), intersections_main_straight(19), intersections_main_turns(28), traffic_lights(34), traffic_controller(29), warning_hazard_signals(24), markings_horizontal(74), markings_vertical(5)
- Subagent C — Asosiy manyovrlar (5 topics): starting_manoeuvring(53), lane_position(41), speed_limits(36), overtaking(37), stopping_and_parking(69)
- Subagent D — Maxsus vaziyatlar + Umumiy qoidalar (11 topics): pedestrian_crossings_stops(14), railway_crossings(18), motorways(14), residential_zones(9), slopes(3), public_transport_priority(21), special_vehicle_priority(10), lighting_devices(22), general_rules(41), driver_duties(13), pedestrian_duties(18)
- Subagent E — Maxsus holatlar va tashish + Xavfsizlik (8 topics): towing(28), driver_training(7), passenger_carriage(20), cargo_carriage(16), cyclists_mopeds_animals(12), first_aid(34), safety_basics(69), vehicle_defects(46)

Dispatch all 5 with the Agent tool (general-purpose, `isolation` not needed —
this only writes to `scratch/`), one message, 5 parallel calls, this exact
prompt template per subagent (fill in `<TOPIC LIST>` and `<TOPIC NAME>` per
row from the group above):

```
You're harvesting question text from osonprava.uz for a content audit. No
code changes, no git operations — you're writing plain JSON files.

Log in is already done (the browser session is authenticated as the app's
owner). For each of these topics, in order:

<TOPIC LIST — e.g. "signs_warning (osonprava name 'Ogohlantiruvchi
belgilar', expect 38 questions), signs_priority (osonprava name 'Imtiyoz
belgilari', expect 23 questions), ...">

1. Load /home/sher/Рабочий стол/avtotest/scratch/category-audit/data/category_map.json
   and read code_to_osonprava_name to get each topic's exact osonprava name.
2. Navigate to https://app.osonprava.uz/topics, find the link for that exact
   topic name (case-sensitive, apostrophe style may differ — match on the
   visible text), and open it (its href is /topic-test/<N> for some N — N is
   NOT the same number as our category.sort, don't assume anything about it).
3. On the topic-test page, click question number 1, capture the question
   stem and its answer options via get_page_text, click question number 2,
   capture, and so on through every number shown (they scroll horizontally —
   scroll the number bar right as needed to reach the end). Stop when you've
   captured the topic's expected count (given above) or the numbers run out,
   whichever comes first.
4. Write one JSON file per topic to
   /home/sher/Рабочий стол/avtotest/scratch/category-audit/data/osonprava_raw/<code>.json
   (code = the category code, e.g. signs_warning.json), an array of
   {"index": <the number you clicked, 1-based>, "question": "<stem text>",
   "answers": ["<option 1>", "<option 2>", ...]}. Do not click or record
   which answer is "correct" — we don't need it and clicking one might submit
   the test.
5. If a topic's harvested count doesn't match its expected count, say so
   explicitly in your final report along with the topic — don't silently
   under- or over-deliver.

Report back: how many topics you completed, the count harvested per topic,
and any topic where the count didn't match expectations or the topic link
couldn't be found by name.
```

- [ ] **Step 3: Write the merge/validation script**

```python
#!/usr/bin/env python3
"""Merge per-topic osonprava.uz captures (from the browser-harvest task) into one file.

The harvest task (Task 5 in the plan) clicks through every question in every
osonprava topic and writes one raw file per topic:
  data/osonprava_raw/<our_category_code>.json
  = [{"index": 1, "question": "...", "answers": ["...", "..."]}, ...]

(one raw file per topic our OSONPRAVA_NAME_TO_CODE mapping covers — nothing
is written for `officials_duties`, which has no osonprava counterpart).

This script merges them into data/osonprava_questions.json (flat list, id =
"<code>:<index>") and checks each topic's harvested count against the totals
captured from https://app.osonprava.uz/topics on 2026-08-30/09-03, so a
partial or mis-clicked harvest is caught immediately instead of silently
feeding Stage 1 a hole.
"""
import json
from pathlib import Path

REPO = Path(__file__).resolve().parents[3]
DATA = REPO / "scratch/category-audit/data"
RAW_DIR = DATA / "osonprava_raw"
OUT = DATA / "osonprava_questions.json"

# code -> expected question count, from /topics on 2026-09-03 (see
# raqobatchi-savol-monitoring-bazasi memory). officials_duties intentionally
# absent (no osonprava counterpart, see category_map.json's no_osonprava_match).
EXPECTED_COUNTS = {
    "signs_warning": 38, "signs_priority": 23, "signs_prohibitory": 59,
    "signs_mandatory": 32, "signs_information": 60, "signs_service": 1,
    "signs_additional": 38,
    "intersections_general": 14, "intersections_regulated": 20,
    "intersections_equal": 48, "intersections_main_straight": 19,
    "intersections_main_turns": 28,
    "traffic_lights": 34, "traffic_controller": 29, "warning_hazard_signals": 24,
    "markings_horizontal": 74, "markings_vertical": 5,
    "starting_manoeuvring": 53, "lane_position": 41, "speed_limits": 36,
    "overtaking": 37, "stopping_and_parking": 69,
    "pedestrian_crossings_stops": 14, "railway_crossings": 18, "motorways": 14,
    "residential_zones": 9, "slopes": 3,
    "public_transport_priority": 21, "special_vehicle_priority": 10,
    "lighting_devices": 22,
    "general_rules": 41, "driver_duties": 13, "pedestrian_duties": 18,
    "towing": 28, "driver_training": 7, "passenger_carriage": 20,
    "cargo_carriage": 16, "cyclists_mopeds_animals": 12,
    "first_aid": 34, "safety_basics": 69, "vehicle_defects": 46,
}
NO_OSONPRAVA_MATCH = {"officials_duties"}


def main():
    cat_map = json.loads((DATA / "category_map.json").read_text())
    expected_codes = set(cat_map["code_to_osonprava_name"].keys())
    assert expected_codes == set(EXPECTED_COUNTS.keys()), (
        expected_codes.symmetric_difference(EXPECTED_COUNTS.keys())
    )

    if not RAW_DIR.exists():
        raise SystemExit(f"{RAW_DIR} doesn't exist yet — run the osonprava harvest task first")

    rows = []
    missing, short, over = [], [], []
    for code in sorted(expected_codes):
        f = RAW_DIR / f"{code}.json"
        if not f.exists():
            missing.append(code)
            continue
        items = json.loads(f.read_text())
        expected = EXPECTED_COUNTS[code]
        if len(items) < expected:
            short.append((code, len(items), expected))
        elif len(items) > expected:
            over.append((code, len(items), expected))
        for it in items:
            rows.append({
                "id": f"{code}:{it['index']}",
                "category": code,
                "question": it["question"],
                "answers": it.get("answers", []),
            })

    OUT.write_text(json.dumps(rows, ensure_ascii=False, indent=1))
    print(f"wrote {len(rows)} rows from {len(expected_codes) - len(missing)}/{len(expected_codes)} topics -> {OUT}")
    if missing:
        print(f"MISSING topics (not harvested at all): {missing}")
    if short:
        print(f"SHORT topics (fewer than expected — likely an incomplete click-through): {short}")
    if over:
        print(f"OVER topics (more than expected — check for duplicate captures): {over}")
    if missing or short:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
```

- [ ] **Step 4: Run the merge and fix any gaps**

Run: `python3 scripts/seed/category_audit/06_merge_osonprava.py`
Expected: `wrote <~1209> rows from 41/41 topics -> .../osonprava_questions.json`
with no MISSING/SHORT/OVER lines. If any topic is missing or short,
dispatch one more subagent scoped to just that topic (same prompt template,
one topic) rather than re-running the whole group — don't discard the topics
that already succeeded.

- [ ] **Step 5: Commit the script**

```bash
git add scripts/seed/category_audit/06_merge_osonprava.py
git commit -m "$(cat <<'EOF'
chore(audit): add category-audit Task 5 script (osonprava merge)

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Stage 1 — shortlist candidates

**Files:**
- Create: `scripts/seed/category_audit/05_shortlist_candidates.py`
- Reads: `our_questions.json`, `avtodrom_ref.json`, `ptest_questions.json` + `ptest_tag_map.json`, `osonprava_questions.json` (Tasks 1, 3, 4, 5)
- Produces: `scratch/category-audit/data/stage1_candidates.json`

**Interfaces:**
- Produces: `{ext_id: {current_category, avtodrom: [{ref, category, sim}], ptest: [...], osonprava: [...]}}`, up to 3 candidates per source, only similarity ≥ 0.55. Task 7's subagents read this as their starting point per question.

- [ ] **Step 1: Write the script**

```python
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
```

- [ ] **Step 2: Run it (osonprava must exist by now — run this after Task 5, not before) and verify**

Run: `python3 scripts/seed/category_audit/05_shortlist_candidates.py`
Takes several minutes (all-pairs fuzzy matching over ~1273 × ~3800
candidates — measured 6m35s with only avtodrom+ptest loaded on 2026-09-03,
expect a bit more with osonprava's ~1209 added). Expected: no WARNING line
(osonprava should be found this time), and a "with zero candidates anywhere"
count — 0 was the result on 2026-09-03 with just avtodrom+ptest; if it's
nonzero now, that's fine, just means some questions are genuinely twin-less
everywhere (surviving to a "read alone" decision in Task 7).

Spot-check: `python3 -c "import json; d=json.load(open('scratch/category-audit/data/stage1_candidates.json')); print(json.dumps(d['avtoimtihon-227'], ensure_ascii=False, indent=1))"`
should show `current_category: lighting_devices` next to an avtodrom
candidate at `sim: 1.0` whose `category` is `signs_additional` — the known
miscategorization this whole project exists to catch. If that question's
`category` field no longer contains `lighting_devices` in `our_questions.json`
someone already fixed it by hand; otherwise this mismatch is the pipeline
working.

- [ ] **Step 3: Commit**

```bash
git add scripts/seed/category_audit/05_shortlist_candidates.py
git commit -m "$(cat <<'EOF'
chore(audit): add category-audit Task 6 script (Stage 1 shortlist)

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Stage 2 — first-pass read-and-decide audit

No new script — this task is dispatching 7 subagents against the data Tasks
1-6 already produced, then merging their output.

**Files:**
- Produces: `scratch/category-audit/data/stage2_audit.json`

**Interfaces:**
- Consumes: `our_questions.json`, `stage1_candidates.json`, `avtodrom_ref.json`, `ptest_questions.json` + `ptest_tag_map.json`, `osonprava_questions.json`.
- Produces: a JSON array, each element `{ext_id, current_category, proposed_category, sources_found: [source names with a confirmed pairing], reasoning, confidence: "high"|"medium"|"low"}`. `proposed_category == current_category` when nothing changes. Task 8 reads every row where they differ, plus a 10% sample of the rest.

- [ ] **Step 1: Split our 1273 questions into 7 batches by current category**

Batches (computed 2026-09-03 from `our_questions.json`'s per-category
counts; re-derive if Task 1's totals differ meaningfully, but this split
doesn't need to be exact — it's for readability, not correctness):

| Batch | Categories | ~Questions |
|---|---|---|
| 1 | general_rules, driver_duties, pedestrian_duties, special_vehicle_priority, signs_warning, signs_priority | 141 |
| 2 | signs_prohibitory, signs_mandatory, signs_information, signs_service, signs_additional | 191 |
| 3 | markings_horizontal, markings_vertical, traffic_lights, traffic_controller, warning_hazard_signals | 155 |
| 4 | starting_manoeuvring, lane_position, speed_limits, overtaking | 185 |
| 5 | stopping_and_parking, intersections_general, intersections_regulated, intersections_main_straight, intersections_equal | 192 |
| 6 | intersections_main_turns, pedestrian_crossings_stops, railway_crossings, motorways, residential_zones, slopes, public_transport_priority, lighting_devices, towing, driver_training, passenger_carriage, cargo_carriage, cyclists_mopeds_animals | 206 |
| 7 | officials_duties, vehicle_defects, safety_basics, first_aid | 203 |

- [ ] **Step 2: Dispatch all 7 subagents in parallel (one message, 7 Agent calls)**

Use the `general-purpose` agent type (full tool access, needs `Read`/`Bash`
to load the JSON files). This exact prompt template per subagent, filling in
`<BATCH CATEGORIES>` (the category list + full uz-Latn names from the table
below) and `<OUTPUT FILE>` (`scratch/category-audit/data/stage2_batch_1.json`
through `_7.json`):

**Full category name reference (all 42, for every subagent's context):**

```
general_rules=Umumiy qoidalar, driver_duties=Haydovchilarning umumiy vazifalari,
pedestrian_duties=Piyodalarning umumiy vazifalari,
special_vehicle_priority=Maxsus transport vositalarining imtiyozlari,
signs_warning=Ogohlantiruvchi belgilar, signs_priority=Imtiyoz belgilari,
signs_prohibitory=Taqiqlovchi belgilar, signs_mandatory=Buyuruvchi belgilar,
signs_information=Axborot-ko'rsatgich belgilari, signs_service=Servis belgilari,
signs_additional=Qo'shimcha axborot belgilari, markings_horizontal=Yotiq chiziqlar,
markings_vertical=Tik chiziqlar, traffic_lights=Svetofor ishoralari,
traffic_controller=Tartibga soluvchining ishoralari,
warning_hazard_signals=Ogohlantiruvchi va avariya (xavf-xatar) ishoralari,
starting_manoeuvring=Harakatlanishni boshlash, manyovr qilish,
lane_position=Qatnov qismida joylashuv, speed_limits=Harakatlanish tezligi,
overtaking=Quvib o'tish, stopping_and_parking=To'xtash va to'xtab turish,
intersections_general=Chorrahalarda harakatlanish,
intersections_regulated=Tartibga solingan chorrahalar,
intersections_main_straight=Tartibga solinmagan: asosiy yo'l to'g'riga,
intersections_equal=Tartibga solinmagan: teng ahamiyatli,
intersections_main_turns=Tartibga solinmagan: asosiy yo'l o'zgaradi,
pedestrian_crossings_stops=Piyodalar o'tish joyi va bekatlar,
railway_crossings=Temir yo'l kesishmalari orqali harakatlanish,
motorways=Avtomagistrallarda harakatlanish,
residential_zones=Turar joy dahalarida harakatlanish,
slopes=Tik balandlik va nishabliklarda harakatlanish,
public_transport_priority=Yo'nalishli transport vositalarining imtiyozlari,
lighting_devices=Tashqi yoritish asboblaridan foydalanish,
towing=Mexanik transport vositalarini shatakka olish,
driver_training=Transport vositalarini boshqarishni o'rgatish,
passenger_carriage=Odam tashish, cargo_carriage=Yuk tashish,
cyclists_mopeds_animals=Velosiped, moped va aravalar,
officials_duties=Mansabdor shaxslar majburiyatlari,
vehicle_defects=Transport vositalaridan foydalanishni taqiqlovchi shartlar,
safety_basics=Harakat xafsizligi asoslari, first_aid=Birinchi tibbiy yordam
```

**Prompt template:**

```
You're auditing whether each of a set of driving-theory exam questions is
filed under the right topic, out of our fixed 42 YHQ-chapter topics. This is
a content-quality task — read Uzbek text and traffic-law logic carefully, no
code changes, no git operations.

The 42 topics (code=name): <FULL CATEGORY NAME REFERENCE — paste the block
above>

Your batch is these current categories: <BATCH CATEGORIES with names>.

Load these files (absolute paths):
- /home/sher/Рабочий стол/avtotest/scratch/category-audit/data/our_questions.json
  — find every row whose "category" is one of your batch's categories. That
  is your worklist (~<N> questions).
- /home/sher/Рабочий стол/avtotest/scratch/category-audit/data/stage1_candidates.json
  — keyed by ext_id, up to 3 candidate twins per source (avtodrom, ptest,
  osonprava) with a similarity score. These are NOT confirmed matches — they
  are what to go look at.
- /home/sher/Рабочий стол/avtotest/scratch/category-audit/data/avtodrom_ref.json
  — look up a candidate's full text + "citation" (an actual YHQ article
  quote) by its avtodrom_id.
- /home/sher/Рабочий стол/avtotest/scratch/category-audit/data/ptest_questions.json
  — look up a candidate's full text by its ptest id.
- /home/sher/Рабочий стол/avtotest/scratch/category-audit/data/osonprava_questions.json
  — look up a candidate's full text by its id (may be missing some topics if
  that harvest didn't complete — treat absence as "no osonprava signal" for
  those, don't block on it).

For every question in your worklist, in order:

1. Read the question and all its answers (our_questions.json has the full
   text). Understand what specific rule or fact it's testing.
2. For each of that question's candidates in stage1_candidates.json, open
   the candidate's full text from the matching reference file. A high
   similarity score is a hint, not proof — confirm or reject by reading:
   does this candidate test the SAME rule (same scenario, same correct
   answer logic), not just similar wording? Reject candidates that merely
   share vocabulary. Never use the presence or absence of an image to
   accept or reject a pairing — the same rule gets illustrated differently
   on different sites.
3. Gather the category of every candidate you confirmed as a genuine twin.
4. Decide the question's true topic:
   - If every confirmed signal (and there may be zero, one, two, or three)
     agrees with the question's current_category (from stage1_candidates.json)
     → keep it. This should be the common case — don't overthink these,
     move fast.
   - If any confirmed signal disagrees with the current category → read the
     question, its answers, and — if you confirmed an avtodrom twin — its
     "citation" field (a real YHQ article quote) closely, and decide from
     what the rule actually says. The avtodrom citation carries the most
     weight because it names a real law article. Do NOT just go with
     whichever category 2 of 3 sources agree on if that agreement looks
     wrong against the actual rule — sources can be confidently wrong (a
     known example: ptest itself once tagged a right-of-way overtaking
     question under "lighting_devices").
   - If you found zero confirmed signals anywhere → decide from reading the
     question and the 42 topic definitions alone, same standard as any
     hand-classified question would need.
5. Write one row to your output array (even for "keep unchanged" rows —
   every question in your worklist needs an entry):
   {"ext_id": ..., "current_category": ..., "proposed_category": <same as
   current if unchanged, otherwise your correction>, "sources_found":
   [list of source names you confirmed a twin in, e.g. ["avtodrom", "ptest"]
   or [] if none], "reasoning": "<one sentence naming what the question
   actually tests and why that topic fits — not 'sources agreed'>",
   "confidence": "high"|"medium"|"low"}

Confidence guide: "high" = a confirmed avtodrom twin with a clear citation
supporting your call, or the rule is unambiguous on a plain reading; "medium"
= confirmed signal(s) support your call but the citation is vague, ambiguous,
or absent, or the question could plausibly fit two topics; "low" = you're
genuinely unsure and are making a defensible best guess — flag these
explicitly in reasoning too.

Write your full output array to
/home/sher/Рабочий стол/avtotest/scratch/category-audit/data/<OUTPUT FILE>
as pretty-printed JSON (indent=1, ensure_ascii=False if you write it via
Python — this repo's JSON files use raw Uzbek/Cyrillic/Russian text, not
\uXXXX escapes).

Report back: how many questions in your worklist, how many you propose
changing, and list every changed ext_id with its current -> proposed
category (a two-line summary is fine, the file has the full reasoning).
```

- [ ] **Step 3: Verify each batch output before merging**

For each of the 7 output files, confirm:
- `python3 -c "import json; print(len(json.load(open('scratch/category-audit/data/stage2_batch_N.json'))))"` matches that batch's expected worklist size (count questions in `our_questions.json` whose category is in that batch's list — don't trust the subagent's self-reported count blindly).
- Every row has all 6 fields (`ext_id`, `current_category`, `proposed_category`, `sources_found`, `reasoning`, `confidence`) and `confidence` is one of the three allowed values.
- No `ext_id` appears in more than one batch file (batches are a partition, not overlapping) and none from that batch's categories are missing.

If a batch is incomplete or malformed, re-dispatch just that batch (same
prompt, that batch only) — don't discard batches that already succeeded.

- [ ] **Step 4: Merge into one file**

```bash
python3 -c "
import json
from pathlib import Path
data_dir = Path('scratch/category-audit/data')
merged = []
for i in range(1, 8):
    merged.extend(json.loads((data_dir / f'stage2_batch_{i}.json').read_text()))
assert len(merged) == len(json.loads((data_dir / 'our_questions.json').read_text())), \
    f'{len(merged)} != {len(json.loads((data_dir / \"our_questions.json\").read_text()))}'
(data_dir / 'stage2_audit.json').write_text(json.dumps(merged, ensure_ascii=False, indent=1))
changed = sum(1 for r in merged if r['proposed_category'] != r['current_category'])
print(f'merged {len(merged)} rows, {changed} proposed changes')
"
```

Expected: the assert passes (every one of our questions got exactly one
Stage 2 row) and a changed-count prints — this is the number Task 8 and
Task 10's report will work with.

---

## Task 8: Stage 3 — independent re-audit

**Files:**
- Produces: `scratch/category-audit/data/stage3_reaudit.json`

**Interfaces:**
- Consumes: `stage2_audit.json` (Task 7).
- Produces: a JSON array, each element `{ext_id, stage2_proposed, stage3_verdict: "agree"|"dissent", stage3_category (if dissent, the reader's own proposed category), reasoning}`. Task 9 reads every `dissent` row.

- [ ] **Step 1: Build the re-audit worklist**

```bash
python3 -c "
import json, random
from pathlib import Path
data_dir = Path('scratch/category-audit/data')
stage2 = json.loads((data_dir / 'stage2_audit.json').read_text())
changed = [r for r in stage2 if r['proposed_category'] != r['current_category']]
unchanged = [r for r in stage2 if r['proposed_category'] == r['current_category']]
random.seed(20260903)
control_sample = random.sample(unchanged, max(1, len(unchanged) // 10))
worklist = [r['ext_id'] for r in changed] + [r['ext_id'] for r in control_sample]
(data_dir / 'stage3_worklist.json').write_text(json.dumps({
    'changed': [r['ext_id'] for r in changed],
    'control_sample': [r['ext_id'] for r in control_sample],
    'all': worklist,
}, ensure_ascii=False, indent=1))
print(f'{len(changed)} changed + {len(control_sample)} control sample = {len(worklist)} to re-audit')
"
```

Expected: prints a count. Save this — it's how many questions Step 2's
subagents will split.

- [ ] **Step 2: Split the worklist and dispatch fresh subagents**

Split `stage3_worklist.json`'s `all` list into groups of ~100-150 (however
many subagents that takes — likely 2-4 given Stage 3's worklist is much
smaller than Stage 2's full 1273). Dispatch with the Agent tool
(`general-purpose`), one message, all groups in parallel. Each subagent gets
**no visibility into Stage 2's reasoning or proposed category** — it
re-derives the answer from scratch, the same way Task 7's subagents did:

```
You're independently re-checking a set of driving-theory exam category
corrections. This is a second, blind opinion — you have NOT seen and must
NOT look at any prior audit's reasoning or proposed category, only the
question and the same reference sources used before. No code changes, no
git operations.

The 42 topics (code=name): <FULL CATEGORY NAME REFERENCE — same block as Task 7>

Your ext_ids to re-check: <LIST>

Load these files (absolute paths):
- /home/sher/Рабочий стол/avtotest/scratch/category-audit/data/our_questions.json
  — look up each ext_id's full question + answers + ITS CURRENT category
  (the field literally named "category" — this is the pre-audit value, use
  it only to report your verdict against, not as a hint toward the answer).
- /home/sher/Рабочий стол/avtotest/scratch/category-audit/data/stage1_candidates.json
  — candidate twins per source, same shortlist Stage 2 used.
- /home/sher/Рабочий стол/avtotest/scratch/category-audit/data/avtodrom_ref.json
  /home/sher/Рабочий стол/avtotest/scratch/category-audit/data/ptest_questions.json
  /home/sher/Рабочий стол/avtotest/scratch/category-audit/data/osonprava_questions.json
  — full text (and avtodrom's citation) for the candidates, same as before.

Do NOT open scratch/category-audit/data/stage2_audit.json or any
stage2_batch_*.json file — that would defeat the point of an independent
check.

For each ext_id: read the question and answers, confirm or reject each
candidate twin by reading (not by similarity score, not by image), weigh
avtodrom's citation most heavily when one is confirmed, and decide the
question's true topic exactly as described in Task 7's method (same rules:
sources can be wrong, majority isn't automatic, decide from what the rule
says).

Then compare your own conclusion to the question's CURRENT category (already
loaded from our_questions.json):
- Your conclusion matches current_category exactly as the prior audit
  proposed changing it AWAY from → note you'd have kept it (a dissent from
  Stage 2's change).
- Your conclusion differs from current_category the same way a change is
  needed → note what topic you land on.
Since you can't see Stage 2's proposal, express your verdict as your own
independent proposed_category, and separately note in reasoning whether you
think the CURRENT (pre-audit) category was right or wrong and why.

Write one row per ext_id to
/home/sher/Рабочий стол/avtotest/scratch/category-audit/data/<OUTPUT FILE>:
{"ext_id": ..., "your_proposed_category": ..., "reasoning": "<one sentence,
same standard as before — name what the question tests>"}

Report back: your proposed changes (ext_id, current -> your proposal) and
overall count.
```

- [ ] **Step 3: Reconcile against Stage 2 and write stage3_reaudit.json**

```bash
python3 -c "
import json
from pathlib import Path
data_dir = Path('scratch/category-audit/data')
stage2_by_id = {r['ext_id']: r for r in json.loads((data_dir / 'stage2_audit.json').read_text())}

# Load and merge every stage3 batch file you dispatched in Step 2 — adjust
# this glob/list to match whatever filenames you actually used.
stage3_batches = sorted(data_dir.glob('stage3_batch_*.json'))
stage3_rows = []
for f in stage3_batches:
    stage3_rows.extend(json.loads(f.read_text()))

worklist = json.loads((data_dir / 'stage3_worklist.json').read_text())['all']
by_id = {r['ext_id']: r for r in stage3_rows}
missing = set(worklist) - set(by_id)
if missing:
    raise SystemExit(f'{len(missing)} worklist ext_ids missing from stage3 output: {sorted(missing)}')

out = []
for ext_id in worklist:
    s2 = stage2_by_id[ext_id]
    s3 = by_id[ext_id]
    agree = s3['your_proposed_category'] == s2['proposed_category']
    out.append({
        'ext_id': ext_id,
        'current_category': s2['current_category'],
        'stage2_proposed': s2['proposed_category'],
        'stage3_proposed': s3['your_proposed_category'],
        'verdict': 'agree' if agree else 'dissent',
        'stage2_reasoning': s2['reasoning'],
        'stage3_reasoning': s3['reasoning'],
    })

(data_dir / 'stage3_reaudit.json').write_text(json.dumps(out, ensure_ascii=False, indent=1))
dissents = [r for r in out if r['verdict'] == 'dissent']
print(f'{len(out)} re-audited, {len(dissents)} dissents (need main-session escalation)')
"
```

Expected: prints a count of dissents. That number is Task 9's workload — it
should be small (Stage 2 and Stage 3 agreeing most of the time is the
healthy outcome; a very high dissent rate would suggest the prompt or the
reference data has a systemic problem worth stopping to investigate before
continuing).

---

## Task 9: Resolve escalations and finalize accepted changes

**Files:**
- Produces: `scratch/category-audit/data/escalations.json`, `scratch/category-audit/data/accepted_changes.json`

**Interfaces:**
- Consumes: `stage3_reaudit.json` (Task 8).
- Produces: `accepted_changes.json` = the exact input Task 10's `07_apply_changes.py` expects: `[{"ext_id", "from", "to"}]`, one entry per question whose category actually changes (Stage 2 proposed a change AND either Stage 3 agreed or the escalation resolved in favor of changing).

- [ ] **Step 1: Read every dissent personally (main session, not a subagent)**

```bash
python3 -c "
import json
d = json.load(open('scratch/category-audit/data/stage3_reaudit.json'))
dissents = [r for r in d if r['verdict'] == 'dissent']
print(json.dumps(dissents, ensure_ascii=False, indent=1))
"
```

For each dissent, read the actual question (`our_questions.json`), its
candidates (`stage1_candidates.json`, `avtodrom_ref.json` for the citation),
Stage 2's reasoning, and Stage 3's reasoning, then decide yourself which one
is right — or a third answer if both readings missed something. Write your
resolution for every dissent to `scratch/category-audit/data/escalations.json`:

```json
[
  {"ext_id": "avtoimtihon-227", "stage2_proposed": "signs_additional",
   "stage3_proposed": "signs_additional", "final_category": "signs_additional",
   "resolution_note": "Both stages agree and the avtodrom exact-text twin's citation (YHQ appendix 3.27, a road sign) confirms it — the current lighting_devices tag was simply wrong."}
]
```

(That example is an *agreement* shown for format only — Step 1 is only for
rows where `verdict == "dissent"`; don't re-litigate rows Stage 2 and Stage 3
already agreed on.)

- [ ] **Step 2: Build the final accepted-changes list**

```bash
python3 -c "
import json
from pathlib import Path
data_dir = Path('scratch/category-audit/data')
stage2 = {r['ext_id']: r for r in json.loads((data_dir / 'stage2_audit.json').read_text())}
stage3 = {r['ext_id']: r for r in json.loads((data_dir / 'stage3_reaudit.json').read_text())}
escalations_path = data_dir / 'escalations.json'
escalations = {r['ext_id']: r for r in json.loads(escalations_path.read_text())} if escalations_path.exists() else {}

worklist_ext_ids = set(json.loads((data_dir / 'stage3_worklist.json').read_text())['changed'])

accepted = []
for ext_id in worklist_ext_ids:
    s2 = stage2[ext_id]
    s3 = stage3.get(ext_id)
    if ext_id in escalations:
        final = escalations[ext_id]['final_category']
    elif s3 and s3['verdict'] == 'agree':
        final = s3['stage3_proposed']
    elif s3 is None:
        # wasn't in the stage3 worklist for some reason — shouldn't happen
        # for a row that came from 'changed', investigate before proceeding
        raise SystemExit(f'{ext_id}: proposed change with no stage3 verdict and no escalation')
    else:
        raise SystemExit(f'{ext_id}: dissent with no escalation resolution — finish Step 1 first')

    if final != s2['current_category']:
        accepted.append({'ext_id': ext_id, 'from': s2['current_category'], 'to': final})

(data_dir / 'accepted_changes.json').write_text(json.dumps(accepted, ensure_ascii=False, indent=1))
print(f'{len(accepted)} accepted changes -> accepted_changes.json')
"
```

Expected: prints the final change count (at or below Stage 2's original
proposed-change count — some proposals get walked back by Stage 3 agreement
that the current category was actually fine, some get resolved differently
by escalation).

---

## Task 10: Consolidate report, apply, and validate

**Files:**
- Create: `scripts/seed/category_audit/07_apply_changes.py`
- Modify: `backend/seed/avtoimtihon/data.json`, `backend/seed/avtoimtihon/assignments.json`
- Produces: `scratch/category-audit/data/report.md`

**Interfaces:**
- Consumes: `accepted_changes.json` (Task 9).

- [ ] **Step 1: Write the apply script**

```python
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
```

- [ ] **Step 2: Run it**

Run: `python3 scripts/seed/category_audit/07_apply_changes.py`
Expected: `applied <N> category changes to data.json + assignments.json`
followed by a per-target-category breakdown. This modifies the two seed
files in the working tree — that's the intended effect, not a side effect.

- [ ] **Step 3: Verify the diff is category-field-only**

Run: `git diff --stat backend/seed/avtoimtihon/`
Expected: two files changed, and the insertion/deletion counts are roughly
2× the applied-change count (one line changes per `"category": "..."` field
per file — NOT a full-file reformat; if either file's diff is in the tens of
thousands of lines, something reformatted the whole file — check
`07_apply_changes.py`'s `json.dump` calls match the exact `indent=1`/`indent=2`
and trailing-newline behavior documented in its docstring, the same mistake
made and caught earlier today when adding the 8 ptest questions).

- [ ] **Step 4: Validate with the real importer**

```bash
mkdir -p backend/cmd/tmpvalidateseed
cat > backend/cmd/tmpvalidateseed/main.go << 'EOF'
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"avtotest.uz/backend/internal/importer"
)

func main() {
	raw, err := os.ReadFile("seed/avtoimtihon/data.json")
	if err != nil {
		fmt.Println("read error:", err)
		os.Exit(1)
	}
	var ds importer.Dataset
	if err := json.Unmarshal(raw, &ds); err != nil {
		fmt.Println("unmarshal error:", err)
		os.Exit(1)
	}
	fmt.Println("questions:", len(ds.Questions), "variants:", len(ds.Variants))
	issues := importer.Validate(ds)
	if len(issues) == 0 {
		fmt.Println("OK: 0 issues")
		return
	}
	fmt.Printf("%d issues:\n", len(issues))
	for _, is := range issues {
		fmt.Printf("  [%s] %s %s: %s\n", is.Code, is.Entity, is.ID, is.Detail)
	}
}
EOF
docker run --rm -v "$PWD":/src -v avtotest-gomod:/go/pkg/mod \
  -w /src/backend golang:1.27 go run ./cmd/tmpvalidateseed
rm -rf backend/cmd/tmpvalidateseed
```

Expected: `OK: 0 issues` (a category *reassignment* can't introduce a
structural issue on its own — `Validate` checks things like answer counts
and known category codes, both untouched by this pipeline — so a non-zero
result here means the apply script corrupted something and needs
investigating before continuing, not a data-content problem to work around).

- [ ] **Step 5: Write the human-facing report**

```bash
python3 -c "
import json
from pathlib import Path
data_dir = Path('scratch/category-audit/data')
accepted = json.loads((data_dir / 'accepted_changes.json').read_text())
stage2 = {r['ext_id']: r for r in json.loads((data_dir / 'stage2_audit.json').read_text())}
escalations_path = data_dir / 'escalations.json'
escalations = {r['ext_id']: r for r in json.loads(escalations_path.read_text())} if escalations_path.exists() else {}
our = {r['ext_id']: r for r in json.loads((data_dir / 'our_questions.json').read_text())}

lines = ['# 42-topic category audit report', '', f'{len(accepted)} questions moved to a different topic.', '']
lines.append('| ext_id | before | after | reasoning |')
lines.append('|---|---|---|---|')
for c in sorted(accepted, key=lambda x: x['ext_id']):
    reasoning = escalations.get(c['ext_id'], {}).get('resolution_note') or stage2[c['ext_id']]['reasoning']
    stem = our[c['ext_id']]['texts']['uz-Latn'][:60].replace('|', '\\\\|')
    lines.append(f\"| {c['ext_id']} ({stem}...) | {c['from']} | {c['to']} | {reasoning.replace('|', '\\\\|')} |\")

by_target = {}
for c in accepted:
    by_target.setdefault(c['to'], []).append(c['ext_id'])
by_source = {}
for c in accepted:
    by_source.setdefault(c['from'], []).append(c['ext_id'])
lines += ['', '## Per-topic net change', '', '| topic | net |', '|---|---|']
for code in sorted(set(by_target) | set(by_source)):
    net = len(by_target.get(code, [])) - len(by_source.get(code, []))
    lines.append(f'| {code} | {net:+d} |')

(data_dir / 'report.md').write_text('\n'.join(lines))
print(f'wrote {data_dir / \"report.md\"}')
"
```

Expected: `wrote .../report.md`. Read it before telling the user this task is
done — this file, plus `git diff backend/seed/avtoimtihon/`, is what the
user reviews to decide the rollout question deferred in the spec's Risk
section (ship as-is vs. pair with a `category_mastery` recompute /
`practice_cursor` clamp migration for the affected topics).

- [ ] **Step 6: Stop — do not commit**

This task, and this plan, end here. The working tree has the two modified
seed files and `scratch/category-audit/data/report.md`
(gitignored — copy its content into the chat or a message to the user, don't
rely on them finding it). Do not run `git add`/`git commit` on
`backend/seed/avtoimtihon/*` and do not touch deploy — those are the user's
call once they've read the report, per the spec's explicit scope boundary.

---

## Self-review notes

- **Spec coverage:** every spec section maps to a task — reference sources
  (Tasks 1-6), matching method (Task 6's shortlist + every subagent prompt's
  Step 2 instruction to confirm-by-reading), decision rule (spelled out
  verbatim in the Task 7/8 prompts), 4-stage pipeline (Tasks 7/8/9/10 are
  Stages 2/3/(3+escalation)/4; Task 6 is Stage 1; Tasks 1-5 are Stage 0), risk
  section (Task 10 Step 5's report is what the deferred rollout decision
  reads), out-of-scope items (no task imports questions, touches the schema,
  or commits/deploys).
- **Type consistency:** `ext_id`, `category`/`current_category`/
  `proposed_category`, and the `{ref, category, sim}` candidate shape are
  used identically across Tasks 6-10's scripts and prompts — checked by
  actually running Tasks 1, 2, 3, 4, and 6 end-to-end against each other
  during plan-writing (2026-09-03: 1273 our-questions, 1264 avtodrom rows,
  1309 ptest questions, a working shortlist with a confirmed
  `avtoimtihon-227` mismatch), and Task 10's apply script against a scratch
  copy of the real seed files (produced a 2-line diff, not a reformat).
- **Placeholder scan:** no TBD/TODO; the one open, deliberate placeholder is
  Task 5 Step 2's harvesting subagent group sizes, which are estimates
  (osonprava's per-question harvest wasn't run during planning, only its
  interaction pattern and per-topic totals were confirmed) — Task 5 Step 4's
  count-validation against `EXPECTED_COUNTS` is what catches a bad split, not
  the split itself needing to be exact.
