#!/usr/bin/env python3
"""Merge per-topic osonprava.uz captures (from the browser-harvest task) into one file.

The harvest task (Task 4 in the plan) clicks through every question in every
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
