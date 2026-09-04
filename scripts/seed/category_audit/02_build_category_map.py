#!/usr/bin/env python3
"""Build the canonical category map: our 42 codes <-> avtodrom topic # <-> osonprava topic name.

CORRECTED 2026-09-04: an earlier version of this script assumed avtodrom's
`topic` field 1..42 followed the same order as our category.sort (both being
"the 42 YHQ topics in document order"). That assumption was wrong — avtodrom
groups general rules/signals/maneuvers/intersections/special-situations
BEFORE signs, while ours puts signs right after general rules — and a
same-length count list happened to validate anyway because it was checked
against a snapshot read off the site in that same (wrong) assumed order, not
independently. The bug produced nonsense: e.g. avtodrom id=1 (a disabled-
parking sign question, unmistakably signs_additional) was mapped to
cargo_carriage. A Stage 2 audit subagent caught this ("avtodrom's category
field agrees with ours only 16%, even for questions confirmed identical by
text") before any bad data reached the real seed files.

avtodrom's TRUE topic id -> name table lives in its own frontend bundle
(`/home/sher/Рабочий стол/avtodrom/assets/index-BzkTC_eo.js`, a `{id,title,
count}` array literal) — extracted and hand-mapped to our codes by name
below. Verified 2026-09-04: every one of the 42 counts in that JS array
exactly matches a `Counter` over the local questions_uzl.json's `topic`
field, so the JS array's ids ARE the ids the questions use — this is now a
name lookup, exactly like the osonprava one below, not a positional guess.

osonprava.uz topics do NOT share our order either (they're grouped into 9
sections on their /topics page) and are missing our `officials_duties` while
carrying an extra "Taniqli belgilar" topic we don't have — so the osonprava
side is a name lookup table, hand-built from the /topics page text captured
2026-09-03, with two rows deliberately absent (see NO_OSONPRAVA_MATCH /
OSONPRAVA_ONLY below).

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

# avtodrom topic id (from its own app bundle's {id,title,count} array,
# language "uz") -> our category code. Name-matched by hand 2026-09-04.
AVTODROM_TOPIC_TO_CODE = {
    1: "general_rules",
    2: "driver_duties",
    3: "pedestrian_duties",
    4: "special_vehicle_priority",
    5: "traffic_lights",
    6: "traffic_controller",
    7: "warning_hazard_signals",
    8: "starting_manoeuvring",
    9: "lane_position",
    10: "speed_limits",
    11: "overtaking",
    12: "stopping_and_parking",
    13: "intersections_general",
    14: "intersections_regulated",
    15: "intersections_main_straight",
    16: "intersections_equal",
    17: "intersections_main_turns",
    18: "pedestrian_crossings_stops",
    19: "railway_crossings",
    20: "motorways",
    21: "residential_zones",
    22: "slopes",
    23: "public_transport_priority",
    24: "lighting_devices",
    25: "towing",
    26: "driver_training",
    27: "passenger_carriage",
    28: "cargo_carriage",
    29: "cyclists_mopeds_animals",
    30: "officials_duties",
    31: "signs_warning",
    32: "signs_priority",
    33: "signs_prohibitory",
    34: "signs_mandatory",
    35: "signs_information",
    36: "signs_service",
    37: "signs_additional",
    38: "markings_horizontal",
    39: "markings_vertical",
    40: "vehicle_defects",
    41: "safety_basics",
    42: "first_aid",
}
assert len(AVTODROM_TOPIC_TO_CODE) == 42
assert set(AVTODROM_TOPIC_TO_CODE.values()) == set(CATEGORY_ORDER)

# The same {id,title,count} rows, for self-consistency verification against
# the local questions_uzl.json (does topic field N actually hold `count`
# questions?) — independent of any assumption about order, since this checks
# each id's own count against itself, not against a separately-read list.
AVTODROM_TOPIC_COUNTS_2026_09_04 = {
    1: 50, 2: 13, 3: 20, 4: 11, 5: 36, 6: 29, 7: 25, 8: 56, 9: 47, 10: 35,
    11: 35, 12: 74, 13: 14, 14: 21, 15: 19, 16: 48, 17: 30, 18: 15, 19: 18,
    20: 14, 21: 9, 22: 4, 23: 20, 24: 22, 25: 30, 26: 6, 27: 18, 28: 17,
    29: 15, 30: 15, 31: 37, 32: 19, 33: 63, 34: 34, 35: 66, 36: 1, 37: 41,
    38: 74, 39: 5, 40: 49, 41: 75, 42: 34,
}
assert set(AVTODROM_TOPIC_COUNTS_2026_09_04) == set(AVTODROM_TOPIC_TO_CODE)

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

    mismatches = [
        (tid, counts.get(tid, 0), expected)
        for tid, expected in AVTODROM_TOPIC_COUNTS_2026_09_04.items()
        if counts.get(tid, 0) != expected
    ]
    if mismatches:
        raise SystemExit(
            "avtodrom local data's per-topic-id counts no longer match the "
            f"app-bundle snapshot: {mismatches}\n"
            "Re-extract the {id,title,count} array from the current "
            "avtodrom frontend bundle before trusting AVTODROM_TOPIC_TO_CODE."
        )

    mapping = {
        "avtodrom_topic_to_code": {str(k): v for k, v in AVTODROM_TOPIC_TO_CODE.items()},
        "code_to_avtodrom_topic": {v: k for k, v in AVTODROM_TOPIC_TO_CODE.items()},
        "osonprava_name_to_code": OSONPRAVA_NAME_TO_CODE,
        "code_to_osonprava_name": {v: k for k, v in OSONPRAVA_NAME_TO_CODE.items()},
        "no_osonprava_match": sorted(NO_OSONPRAVA_MATCH),
        "osonprava_only_topics": sorted(OSONPRAVA_ONLY),
    }

    assert set(OSONPRAVA_NAME_TO_CODE.values()) | NO_OSONPRAVA_MATCH == set(CATEGORY_ORDER), \
        "every one of our 42 codes must be either mapped to an osonprava name or listed in NO_OSONPRAVA_MATCH"

    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(json.dumps(mapping, ensure_ascii=False, indent=1))
    print(f"avtodrom topic-id map verified self-consistent against {sum(AVTODROM_TOPIC_COUNTS_2026_09_04.values())} local questions")
    print(f"osonprava name map: {len(OSONPRAVA_NAME_TO_CODE)} codes mapped, "
          f"{len(NO_OSONPRAVA_MATCH)} unmapped ({sorted(NO_OSONPRAVA_MATCH)})")
    print(f"wrote {OUT}")


if __name__ == "__main__":
    main()
