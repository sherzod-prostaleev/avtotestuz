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
