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
harvesting script (03_harvest_ptest.py) since it needs the live tag IDs, not
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
