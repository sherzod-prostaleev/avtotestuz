# Category taxonomy proposal for the 1235-question bank

Status: **proposal only** — nothing in the database or importer was changed while producing this
document. All numbers below come from computation over
`backend/seed/avtoimtihon/data.json` (1235 questions, 1219 with an `explanations` entry) plus
web research on the official Uzbekistan traffic rules (YHQ/ПДД) and CIS driving-test market
convention. Where a number is an estimate rather than a literal count, it is marked as such.

## 0. Why this matters / current state

All 1235 questions currently sit under the single fallback category `umumiy` ("Общие вопросы").
`backend/internal/db/migrations/0001_content.up.sql` defines `category` as `(id, code UNIQUE,
sort_order)` with per-locale translations in `category_translation` (`status`
`pending`/`verified`). `backend/internal/db/migrations/0004_learning.up.sql` defines
`category_mastery(profile_id, category_id, mastery, seen, correct)` — the mastery-map / weak-area
feature is already wired to `category_id`, it just has nothing to differentiate on. A `question`
row has exactly **one** `category_id` (not a tag set), so the taxonomy below is designed as a
single-assignment classification, matching the existing schema (no new tables needed).

## 1. Evidence: what data.json actually contains

Schema: top-level keys `categories, sign_groups, signs, questions, variants, explanations`.
`explanations[i].legal_refs` is `null` for every single row (0/1219 populated) — it is **not** a
usable structured signal despite the field existing. The real signal is free-text: each
`explanations[i].blocks.ru[].text` is a short paragraph that, in the majority of cases, contains
a literal citation of the traffic-rules text it's explaining, most often in the pattern
**"Пункта N главы M ПДД, гласит: ..."** (paragraph N of chapter M of the PDD states...) or a
reference to one of the PDD's numbered appendices ("Приложение №1/2/3/4 к ПДД").

### 1.1 Citation-frequency table (chapter references, "глава M ПДД")

Extracted via regex `глав[а-я]*\s+(\d+)\s+ПДД` over all 1219 explanation texts. 603/1219 (49.5%)
explanations contain an explicit chapter citation this way (one explanation can mention more than
one chapter; only the first/primary mention was counted to avoid double counting):

| Ch. | Count | Topic (inferred from the actual cited paragraph text in the data, cross-checked against uzpdd.uz's chapter list) |
|---|---|---|
| 16 | 87 | Unregulated intersections — right-of-way ("уступить дорогу приближающимся справа") |
| 13 | 56 | Stopping and parking |
| 7  | 47 | Traffic-light and traffic-controller signals |
| 9  | 45 | Starting off, maneuvering |
| 10 | 41 | Position of vehicle on the carriageway / lanes |
| 8  | 28 | Warning/emergency signals (indicators, horn, hazard lights) |
| 12 | 27 | Overtaking |
| 24 | 26 | Towing |
| 11 | 22 | Speed |
| 23 | 20 | Use of external lighting devices |
| 18 | 18 | Railway crossings |
| 15 | 17 | Regulated (traffic-light) intersections |
| 27 | 16 | Cargo transport |
| 4  | 15 | Pedestrian duties |
| 26 | 15 | Carrying passengers |
| 19 | 14 | Motorways/highways |
| 2  | 14 | General driver duties |
| 6  | 13 | Special-vehicle privileges (flashing beacons etc.) |
| 14 | 12 | Conduct approaching a blocked/jammed intersection |
| 17 | 12 | Pedestrian crossings & public-transport stops |
| 29 | 10 | Duties of officials/citizens re: road safety, ID marks |
| 22 | 10 | Route (public-transport) vehicle priority |
| 28 | 10 | Bicycles, mopeds, animal-drawn carts |
| 20 | 9  | Residential zones |
| 5  | 6  | Passenger duties |
| 21 | 6  | Steep grades/mountain roads |
| 25 | 4  | Learner-driver instruction |
| 3  | 2  | Driver duties at a traffic accident scene |
| 1  | 1  | General definitions |

**Important caveat**: cross-checking against the current officially published chapter list
(uzpdd.uz, reflecting the 2022 PDD revision, lex.uz doc 5953883) shows a probable **numbering
mismatch** — e.g. the current law lists "driver duties after an accident" as chapter 14, but
this dataset's own citations label that exact content as chapter 3. This suggests the
explanations in data.json were authored against an older PDD chapter numbering than the one
currently in force. **Recommendation: treat the chapter numbers above as internal topic labels
from this dataset, not as verified citations against the current law text.** The topical content
(what the paragraph actually says) is reliable; the chapter *number* alone should not be
re-published to users as a legal citation without a human legal check.

### 1.2 Appendix references

| Appendix | Count | Content |
|---|---|---|
| №1 | 185 | Road signs |
| №3 | 38  | Technical/equipment conditions under which a vehicle may not be operated |
| №2 | 36  | Road markings |
| №4 | 3   | Hazard-class placards for dangerous-cargo vehicles |

241 of these 1219 explanations cite *only* an appendix (no chapter number) — a strong,
unambiguous signal for the signs/markings/technical-equipment categories.

### 1.3 Coverage summary

- 603 explanations → chapter citation (primary signal)
- 241 explanations → appendix citation only
- 375 explanations → **no extractable chapter or appendix citation** (30.8% of explanations)
- 16 questions → no `explanations` entry at all (1.3% of all 1235 questions)
- **Total needing non-rule-based classification: 391 / 1235 = 31.7%**

For the 375 "no citation" explanations, a non-exclusive keyword scan of the same text found:
`signs_or_markings`-type words (знак/разметк) in 187, `parking_stop` words in 62,
`intersection_priority` words in 50, `technical_equipment` words in 48, `braking_physics`
words (тормозн/занос/инерц) in 31, `overtaking` words in 29, `speed` words in 28, `first_aid`
words (кровотечен/перелом/жгут/шок/etc.) in 17, with 84 not matching any of these buckets. This
is not a clean partition (a text can match several keyword groups), but it shows the uncited
residue **skews toward signs/markings even more than the cited set does**, and confirms a small
but real first-aid / vehicle-dynamics cluster that essentially never cites a chapter number (its
explanations are narrated procedure, not rule quotations) — this is the main reason it needs its
own category despite low direct citation counts.

Separately: 715/1235 questions (58%) carry an `image`. This is **not** a reliable stand-alone
proxy for "is a sign-recognition question" — intersection-priority scenarios, lane diagrams, and
maneuvering situations are illustrated just as often as signs are.

## 2. Market convention (web research)

- The current Uzbekistan PDD (approved 2022, in force since May 2022, lex.uz doc 5953883) is
  organized into ~29 numbered chapters, matching the chapter range found in the citations above.
- Uzbek/CIS driving-test products (avto-imtihon.uz, 24pdd.uz, pddtest.uz, and the RU convention
  referenced by avto-russia.ru's "билеты по темам") converge on a mid-teens number of *topic*
  groupings for practice mode, distinct from the 20-question random "ticket/variant" mode: signs
  (often signs+markings together), intersections/priority, stopping & parking, overtaking &
  speed, signals, pedestrians & public transport, towing, cargo/passenger carriage, special zones
  (railway/highway/residential), vehicle technical condition, and a first-aid/accident-scene
  bucket. The proposal below matches this convention rather than exposing all 29 raw PDD chapters
  as user-facing categories (29 categories would be too granular for a "weak area" UX and would
  fragment already-small chapters like ch.1/ch.3/ch.5 into single-digit-question categories).

## 3. Proposed taxonomy — 13 categories

Sort order matches estimated size, descending. "Cited" = hard count of explanations whose
citation regex/appendix match fell into this bucket (§1.1/1.2, no double counting across
categories). "Estimated total" is a *range*, obtained by grossing the cited count up by the
dataset-wide uncited ratio (×~1.43) and then widening/shifting the range using the §1.3 keyword
evidence for categories where the uncited residue is known to skew away from proportional (signs,
parking, first aid especially). These ranges are honest estimates, not measured counts — the
real distribution can only be established once the LLM classification pass (see §4) runs.

| # | code | uz-Latn | uz-Cyrl | ru | Cited (hard) | Estimated total |
|---|---|---|---|---|---|---|
| 1 | `road_signs_markings` | Yo'l belgilari va chizig'i | Йўл белгилари ва чизиғи | Дорожные знаки и разметка | 221 (App.1+2) | 300–420 |
| 2 | `priority_intersections` | Chorrahalar va yo'l ustunligi | Чорраҳалар ва йўл устунлиги | Перекрёстки и преимущество проезда | 116 (ch.14+15+16) | 140–190 |
| 3 | `maneuvering_lane_position` | Manyovr va yo'lda joylashuv | Манёвр ва йўлда жойлашув | Манёврирование и расположение на проезжей части | 86 (ch.9+10) | 100–140 |
| 4 | `vehicle_equipment_lighting` | Transport vositasi jihozi va yorug'lik | Транспорт воситаси жиҳози ва ёруғлик | Техническое состояние ТС и освещение | 86 (ch.8+23, App.3) | 100–150 |
| 5 | `stopping_parking` | To'xtash va to'xtab turish | Тўхташ ва тўхтаб туриш | Остановка и стоянка | 56 (ch.13) | 80–120 |
| 6 | `overtaking_speed` | Quvib o'tish va tezlik | Қувиб ўтиш ва тезлик | Обгон и скорость движения | 49 (ch.11+12) | 70–110 |
| 7 | `pedestrians_public_transport` | Piyodalar, yo'lovchilar va yo'nalishli transport | Пиёдалар, йўловчилар ва йўналишли транспорт | Пешеходы, пассажиры и маршрутный транспорт | 53 (ch.4+5+17+22+28) | 60–90 |
| 8 | `special_road_zones` | Maxsus yo'l uchastkalari | Махсус йўл участкалари | Особые участки дорог (ж/д переезды, автомагистрали, жилые зоны, спуски) | 47 (ch.18+19+20+21) | 50–80 |
| 9 | `traffic_signals_gestures` | Svetofor va tartibga soluvchi ishoralari | Светофор ва тартибга солувчи ишоралари | Сигналы светофора и регулировщика | 47 (ch.7) | 55–80 |
| 10 | `towing_special_vehicles` | Shatakka olish va maxsus transport | Шатакка олиш ва махсус транспорт | Буксировка и спецтранспорт | 39 (ch.6+24) | 40–65 |
| 11 | `accidents_first_aid_dynamics` | YHH, tez tibbiy yordam va tormozlash | ЙТҲ, тез тиббий ёрдам ва тормозлаш | ДТП, первая помощь и динамика торможения | 2 (ch.3) + keyword residue | 40–70 |
| 12 | `cargo_passenger_carriage` | Yuk va odam tashish | Юк ва одам ташиш | Перевозка людей и грузов | 34 (ch.26+27, App.4) | 35–55 |
| 13 | `general_provisions_admin` | Umumiy qoidalar va majburiyatlar | Умумий қоидалар ва мажбуриятлар | Общие положения и обязанности | 29 (ch.1+2+25+29) | 30–55 |

Sum of "cited (hard)" ≈ 865/1219 explanations (≈70%; slightly over §1.1/1.2's raw totals because
a handful of explanations cite more than one chapter/appendix and were counted once per bucket
where the mapping was unambiguous). Sum of estimated-total midpoints is deliberately **not**
forced to equal 1235 — the ranges overlap in their assumptions and true counts will only be known
after classification.

### Category notes / rationale for grouping

- **#1 road_signs_markings**: by far the largest single topic in both the cited data and the
  uncited residue. Kept as one category (not split by sign group — see open questions) matching
  common CIS-app convention.
- **#2 priority_intersections**: includes ch.14 (conduct at a jammed intersection) since its
  sample content ("don't enter an intersection you can't clear") is really a right-of-way/
  courtesy rule, not an accident-duty rule.
- **#4 vehicle_equipment_lighting**: merges "warning/emergency signal devices" (ch.8),
  "external lighting" (ch.23) and Appendix 3 (equipment that must be present/functional) since
  all three are "is the vehicle equipped/lit correctly" questions.
- **#7 pedestrians_public_transport**: folds in ch.28 (bicycles/mopeds/carts, only 10 cited) as a
  non-motorized-road-user subgroup rather than giving it its own single-digit category.
- **#11 accidents_first_aid_dynamics**: the one category whose real size is most uncertain. It
  has almost no direct chapter citations (ch.3, only 2), because its content (medical procedure,
  braking-distance arithmetic) doesn't quote a PDD paragraph — but the keyword scan of the
  uncited residue (17 first-aid-flavored + 31 braking/skid-flavored explanations) plus the
  sampled question texts (e.g. "How do you transport a casualty with a chest-spine injury?",
  "When should CPR begin?") confirm it is a real, recognizable, exam-relevant cluster that market
  apps present as its own topic. Recommend treating this range as low-confidence until the LLM
  pass runs.
- **#13 general_provisions_admin**: catch-all for definitions (ch.1), general driver duties
  (ch.2), learner-driver rules (ch.25), and official/citizen road-safety duties incl. vehicle
  registration & ID marks (ch.29). Expect this to end up as the smallest category and a
  reasonable "misc" home for edge cases the LLM pass is unsure about — better than reviving a
  new `umumiy`-style dumping ground.

## 4. Tagging methodology recommendation

1. **Rule-based first pass (deterministic, ~68–70% of questions)**: re-run the regex logic used
   for this proposal (`глав[а-я]*\s+(\d+)\s+ПДД`, `[Пп]риложени[ея]\s*№?\s*(\d+)`) as a real
   script, map each matched chapter/appendix number to one of the 13 category codes above via a
   static lookup table (the ch.→category mapping in §3), and assign `question.category_id`
   directly for every question whose *first* explanation citation resolves unambiguously. Because
   this is a literal text match with a fixed lookup table, it needs no LLM and no per-question
   human review — only a one-time human sign-off on the ch./appendix → category mapping table
   itself (13 rows, cheap to review).
2. **LLM classification pass for the remaining ~30%** (375 explanations with no extractable
   citation + 16 questions with no explanation at all): prompt an LLM with the 13 categories, their
   scope descriptions from §3, and ~3–5 few-shot examples per category pulled from step 1's
   confidently-tagged questions (so the few-shots come from the same dataset, not invented). Ask
   for a category code plus a confidence score. Do **not** let the LLM invent a 14th category —
   force a choice from the fixed list, with an explicit "genuinely unsure" escape hatch that
   routes to `general_provisions_admin` pending human review, rather than silently guessing.
3. **Quarantine / spot-check**: sample ~10% of the LLM-tagged questions (all "genuinely unsure"
   ones plus a random sample of confident ones) for human review before going live. Given the
   small absolute size here (~40 questions total to review), a full human read of the LLM's
   "genuinely unsure" bucket is affordable and preferable to a smaller random-only sample.
4. **DB write path**: the existing schema needs no changes. Populate the 13 rows in `category`
   (code, sort_order) + `category_translation` (uz-Latn/uz-Cyrl/ru names, `status='pending'`
   until reviewed, then flip to `verified`) via the same `importer.Store` upsert path already
   used for `umumiy` (`UpsertCategory` / `UpsertCategoryTranslation` in
   `backend/internal/importer/store.go`). Then re-import `data.json` with each `CanonQuestion.Category`
   field updated from `"umumiy"` to its resolved code — `Store()` already re-resolves
   `categoryIDs[cq.Category]` per question on every import, so changing the category field in the
   canonical dataset and re-running the importer is sufficient; no new migration or importer code
   is needed. `category_mastery` will start accumulating per-category stats automatically once
   `question.category_id` is diversified, since it already keys off `category_id`.
5. Keep the retired `umumiy` category row around (don't delete) in case quarantined questions
   need a temporary parking category during the review window — but no question should ship to
   users still tagged `umumiy` once the pass completes.

## 5. Open questions for the user

1. **Category count**: is 13 the right granularity, or would you prefer fewer (e.g. merging
   #6 overtaking_speed into #3 maneuvering, or merging #9 traffic_signals into #2
   priority_intersections) for a simpler mastery-map UI, or more (e.g. splitting #1 road signs by
   sign group — warning/prohibitory/mandatory/informational/service, mirroring the `sign_groups`
   table structure already in data.json, which is currently empty but presumably populated
   elsewhere)?
2. **Should `road_signs_markings` be split from `sign_groups`?** data.json declares a
   `sign_groups`/`signs` structure but both arrays are empty in this dataset — worth checking
   whether a populated version exists elsewhere, since if sign-group metadata exists it could let
   #1 be split into 2-3 sub-categories (e.g. "warning & prohibitory signs" vs "informational &
   service signs") without any new guesswork.
3. **Accidents/first-aid sizing risk (category #11)**: this is the category with the widest
   uncertainty band (40–70) because it almost never cites a chapter number. Acceptable to ship
   with wide uncertainty and let the real count settle after the LLM pass, or should a manual
   keyword-based pre-pass (first-aid/medical vocabulary) be run before the LLM pass specifically
   to firm this one up first?
4. **Chapter-numbering mismatch (§1.1 caveat)**: should the eventual explanation text shown to
   users keep citing "глава N ПДД" as-is (as authored in the source dataset), or should legal
   citations be re-verified/re-numbered against the current 2022 PDD text before shipping? This
   is a legal-accuracy question independent of categorization, but was surfaced by this analysis
   and probably needs a decision either way.
5. **Naming conventions**: category names above are draft translations, not run past a native
   uz-Latn/uz-Cyrl copy reviewer — please treat the three-locale names as placeholders pending a
   language check, not final user-facing copy.
