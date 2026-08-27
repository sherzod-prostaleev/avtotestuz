# From 13 categories to the 42 YHQ topics

2026-08-28

## The problem

Practice → Category offers 13 topics. They were invented for this project in
July 2026 (`docs/superpowers/research/2026-07-21-category-taxonomy-proposal.md`)
and they are too coarse to study with: `road_signs_markings` alone holds 337
questions spanning seven different sign families plus both kinds of road
marking. A learner who wants to drill prohibitory signs cannot.

The official YHQ document already has the right granularity — 42 chapters and
appendix sections, from "Umumiy qoidalar" to "Birinchi tibbiy yordam". A second
site the same owner runs, ptest.uz, exposes exactly that structure and tags its
question bank against it.

## What changes, and what deliberately does not

**Changes:** the `category` table is replaced — the 13 rows go, 42 rows arrive,
and every one of the 1260 questions is re-pointed at its new topic. The practice
picker groups the 42 under nine collapsible headings.

**Does not change:** the schema. `question.category_id` stays a single NOT NULL
FK; no multi-tag table is introduced. Exams, tickets, signs practice, mistakes,
placement and Grand Mock are untouched — they do not select by category. The
ordered-walk mechanism from `0068_practice_cursor` keeps working exactly as
specified, on the new topics.

**Out of scope:** importing the questions ptest has and we do not (see
"What we are not taking"), and any change to how topics are authored — the 42
stay seed-defined, with no admin CRUD.

## Where the mapping comes from

Both banks are the same corpus. Of ptest's 1284 questions and our 1260, **1231
pair 1:1** — 1102 because the stem text is word-for-word identical in Uzbek or
in Russian, 129 through a fuzzy pass over the closer of the two languages.
Answer-count distributions match almost exactly (ptest 199/654/303/128 vs
drivergo 203/646/293/118 for 2/3/4/5 options).

Two negative results are worth recording so nobody repeats them:

- **Russian text alone is not enough.** Compared on Russian only, the overlap
  reads 83%, and topics look far emptier than they are. The two sites translated
  independently and both translations drift. Adding Uzbek lifts it to 96%. The
  first version of this analysis under-counted because of this, and was caught
  by a reviewer asking why "Servis belgilari" showed zero questions when
  `avtoimtihon-552` obviously exists.
- **Image hashing does not pair these banks.** All 760 ptest question images were
  downloaded and compared to our 738 with dHash+aHash. Among pairs already known
  good from text, **55% differ by 41+ bits out of 128**: the two sites redrew the
  same artwork on different backgrounds and crops (`avtoimtihon-902` is one sign,
  3.32, white-on-white for us and on dark grey for them; `avtoimtihon-999` is the
  same story reversed). A picture that fails to match therefore proves nothing.

So the assignment is: **1209 questions take the topic from their ptest twin's
tag**, and the remaining **51 were classified by hand** from the YHQ chapter and
sign numbers already present in each question's `legal_refs`
(`YHQ 13-bob 91-band`, `Belgi 3.27`). Citations were considered as a primary
signal and rejected: only 287 of 649 cited questions land in a chapter that maps
to one topic, because ptest splits chapter 16 into three intersection topics on
what the picture shows, not on what the rule says.

Seven of the 51 are marked uncertain and want a human glance at the picture
before or shortly after the migration. They are assigned to a defensible topic
meanwhile, so nothing is left unassigned:

| question | assigned to | why it is uncertain |
|---|---|---|
| `avtoimtihon-1230` | Taqiqlovchi belgilar | 3,5 t dan ortiq yuk avtomobili taqiqi — belgi turini rasmdan tasdiqlash kerak |
| `avtoimtihon-1247` | Axborot-ko‘rsatgich belgilari | velosiped tasmasi belgisi — 5.x deb taxmin, 4.x bo'lishi ham mumkin |
| `avtoimtihon-1249` | To‘xtash va to‘xtab turish | tirbandlikda to'xtash — chorraha konteksti bo'lishi mumkin |
| `avtoimtihon-1252` | Chorrahalarda harakatlanish | rasmga qarab tartibga solinmagan chorraha turi aniqlanadi |
| `avtoimtihon-1260` | Chorrahalarda harakatlanish | rasmga qarab aniqroq mavzu tanlanishi mumkin |
| `avtoimtihon-162` | Buyuruvchi belgilar | Прил.№1; yo'nalish belgisi — 4.x buyuruvchi deb taxmin, rasmga qarash kerak |
| `avtoimtihon-274` | Tartibga solinmagan chorrahalar teng ahamiyatli | YHQ 16-bob 105-band; 16-bob 3 mavzuga bo'linadi, rasmsiz aniqlanmaydi |

## The 42 topics

`sort_order` is the number below — YHQ document order. Codes are new and share
no string with the outgoing 13 (hence `stopping_and_parking`, not
`stopping_parking`): reusing a code would silently merge old and new topics in
analytics events, which key on category code.

Uzbek Latin, Uzbek Cyrillic and Russian names come from ptest's tag records
(`latin` / `cyrillic` / `russian`). `kaa` is intentionally absent — the domain
allows it but the web frontend has no `/kaa` tree, and the outgoing 13 have no
`kaa` rows either.

**General rules and duties** (1–4, 93 questions)

| # | code | uz-Latn | ru | q |
|--:|---|---|---|--:|
| 1 | `general_rules` | Umumiy qoidalar | Общие положения | 51 |
| 2 | `driver_duties` | Haydovchilarning umumiy vazifalari | Общие обязанности водителей | 14 |
| 3 | `pedestrian_duties` | Piyodalarning umumiy vazifalari | Общие обязанности пешеходов | 19 |
| 4 | `special_vehicle_priority` | Maxsus transport vositalarining imtiyozlari | Преимущества специальных транспортных средств | 9 |

**Road signs** (5–11, 248 questions)

| # | code | uz-Latn | ru | q |
|--:|---|---|---|--:|
| 5 | `signs_warning` | Ogohlantiruvchi belgilar | Предупреждающие знаки | 37 |
| 6 | `signs_priority` | Imtiyoz belgilari | Знаки приоритета | 19 |
| 7 | `signs_prohibitory` | Taqiqlovchi belgilar | Запрещающие знаки | 71 |
| 8 | `signs_mandatory` | Buyuruvchi belgilar | Предписывающие знаки | 27 |
| 9 | `signs_information` | Axborot-ko‘rsatgich belgilari | Информационно-указательные знаки | 55 |
| 10 | `signs_service` | Servis belgilari | Сервисные знаки | 1 |
| 11 | `signs_additional` | Qo‘shimcha axborot belgilari | Знаки дополнительной информации | 38 |

**Road markings** (12–13, 72 questions)

| # | code | uz-Latn | ru | q |
|--:|---|---|---|--:|
| 12 | `markings_horizontal` | Yotiq chiziqlar | Горизонтальная разметка | 65 |
| 13 | `markings_vertical` | Tik chiziqlar | Вертикальная разметка | 7 |

**Signals** (14–16, 95 questions)

| # | code | uz-Latn | ru | q |
|--:|---|---|---|--:|
| 14 | `traffic_lights` | Svetofor ishoralari | Сигналы светофора | 37 |
| 15 | `traffic_controller` | Tartibga soluvchining ishoralari | Сигналы регулировщика | 32 |
| 16 | `warning_hazard_signals` | Ogohlantiruvchi va avariya (xavf-xatar) ishoralari | Предупредительные и аварийные сигналы | 26 |

**Driving order** (17–21, 257 questions)

| # | code | uz-Latn | ru | q |
|--:|---|---|---|--:|
| 17 | `starting_manoeuvring` | Harakatlanishni boshlash, manyovr qilish | Начало движения и маневрирование | 64 |
| 18 | `lane_position` | Yo‘lning qatnov qismida transport vositalarining joylashuvi | Расположение транспортных средств на проезжей части | 44 |
| 19 | `speed_limits` | Harakatlanish tezligi | Скорость движения | 33 |
| 20 | `overtaking` | Quvib o‘tish | Обгон | 41 |
| 21 | `stopping_and_parking` | To‘xtash va to‘xtab turish | Остановка и стоянка | 75 |

**Intersections** (22–26, 143 questions)

| # | code | uz-Latn | ru | q |
|--:|---|---|---|--:|
| 22 | `intersections_general` | Chorrahalarda harakatlanish | Движение на перекрестках | 20 |
| 23 | `intersections_regulated` | Tartibga solingan chorrahalar | Регулируемые перекрестки | 20 |
| 24 | `intersections_main_straight` | Tartibga solinmagan chorrahalar asosiy yo‘l yo‘nalishi to‘g‘riga | Нерегулируемые перекрестки с главной дорогой в прямом направлении | 21 |
| 25 | `intersections_equal` | Tartibga solinmagan chorrahalar teng ahamiyatli | Нерегулируемые перекрестки равнозначных дорог | 53 |
| 26 | `intersections_main_turns` | Tartibga solinmagan chorrahalar asosiy yo‘l yo‘nalishi o‘zgarishi | Нерегулируемые перекрестки с изменением направления главной дороги | 29 |

**Special sections and priority** (27–32, 76 questions)

| # | code | uz-Latn | ru | q |
|--:|---|---|---|--:|
| 27 | `pedestrian_crossings_stops` | Piyodalarning o‘tish joylari va yo‘nalishli transport vositalarining bekatlari | Пешеходные переходы и остановки маршрутных транспортных средств | 14 |
| 28 | `railway_crossings` | Temir yo‘l kesishmalari orqali harakatlanish | Движение через железнодорожные переезды | 20 |
| 29 | `motorways` | Avtomagistrallarda harakatlanish | Движение по автомагистралям | 14 |
| 30 | `residential_zones` | Turar joy dahalarida harakatlanish | Движение в жилых зонах | 9 |
| 31 | `slopes` | Tik balandlik va nishabliklarda harakatlanish | Движение на крутых подъемах и спусках | 2 |
| 32 | `public_transport_priority` | Yo‘nalishli transport vositalarining imtiyozlari | Преимущества маршрутных транспортных средств | 17 |

**Vehicle and carriage** (33–40, 172 questions)

| # | code | uz-Latn | ru | q |
|--:|---|---|---|--:|
| 33 | `lighting_devices` | Tashqi yoritish asboblaridan foydalanish | Использование внешних световых приборов | 24 |
| 34 | `towing` | Mexanik transport vositalarini shatakka olish | Буксировка механических транспортных средств | 28 |
| 35 | `driver_training` | Transport vositalarini boshqarishni o‘rgatish | Обучение управлению транспортными средствами | 5 |
| 36 | `passenger_carriage` | Odam tashish | Перевозка людей | 19 |
| 37 | `cargo_carriage` | Yuk tashish | Перевозка грузов | 17 |
| 38 | `cyclists_mopeds_animals` | Velosiped, moped va aravalar harakatlanishiga, shuningdek, hayvonlarni haydab o‘tishga doir qo‘shimcha talablar | Дополнительные требования к движению велосипедов, мопедов и гужевых повозок, а также к перегону животных | 18 |
| 39 | `officials_duties` | Mansabdor shaxslarning va fuqarolarning yo‘l harakati xavfsizligini taminlash, transport vositalarini yo‘lga chiqarish, raqam va taniqli belgilarini o‘rnatish bo‘yicha majburiyatlari | Обязанности должностных лиц и граждан по обеспечению безопасности дорожного движения, выпуску транспортных средств на линию, установке регистрационных и опознавательных знаков | 13 |
| 40 | `vehicle_defects` | Transport vositalaridan foydalanishni taqiqlovchi shartlar | Условия, запрещающие эксплуатацию транспортных средств | 48 |

**Safety and first aid** (41–42, 104 questions)

| # | code | uz-Latn | ru | q |
|--:|---|---|---|--:|
| 41 | `safety_basics` | Harakat xafsizligi asoslari | Основы безопасности дорожного движения | 70 |
| 42 | `first_aid` | Birinchi tibbiy yordam | Первая медицинская помощь | 34 |

## The migration

One migration, in this order. Steps 1–3 must precede step 4 or the delete
fails.

1. **Insert** the 42 `category` rows and their 126 `category_translation` rows
   (3 locales × 42), `status = 'verified'` to match the outgoing rows —
   `ListCategories` joins on `status = 'verified'` and would render blank names
   otherwise.
2. **Re-point** `question.category_id`, matching on `source_ext_id`. The
   assignment is a literal 1260-row `VALUES` list, generated from
   `backend/seed/avtoimtihon/assignments.json` — the same file the converter
   reads, so the migration and a fresh import cannot disagree. That file is
   rewritten as part of this work (see "Seed and converter") and is the single
   source of truth for which question belongs to which topic; the migration
   embeds a snapshot of it rather than reading it at runtime.
   `source_ext_id` is UNIQUE, so the match is exact. The migration must assert
   `SELECT count(*) FROM question WHERE category_id IN (<old 13>)` is 0
   afterwards, and fail loudly if not.
3. **Null out** `exam_session.category_id` where it points at an outgoing
   category. This FK has neither `ON DELETE CASCADE` nor `SET NULL`, so
   historical practice sessions would block step 4 outright in production. It is
   safe: the column is written on insert and never read back — no query in
   `backend/internal/db/queries/` selects it.
4. **Delete** the 13 old `category` rows. `category_translation`,
   `practice_cursor` and `category_mastery` cascade away with them.

**Accepted data loss**, agreed before writing this:

- `practice_cursor` — every profile's "where the class got to" resets to 0,
  including the shadow profiles on B2B classroom stations. Unavoidable and
  arguably correct: "signs 45/337" has no meaningful successor once signs are
  seven topics. Schools should be told before the deploy.
- `category_mastery` — per-topic mastery resets, and the stats page starts
  showing 42 rows instead of 13. Overall readiness stays well-defined because
  `learning.Service.Stats` weights by category question count, but every user's
  number will move on first recompute.
- `exam_session.category_id` on historical rows — nothing displays it.

`question_memory`, answers, sessions and their scores are all untouched.

## Seed and converter

Production is migrated in place; the importer cannot be re-run (session FKs
already reference the question rows). But the seed pipeline must produce the
same result as production, or the next fresh environment diverges:

- `backend/cmd/convertavtoimtihon/categories.go` — `categoryDefs` becomes the 42
  entries above. `chapterToCategory` and `appendixToCategory` are deleted along
  with `classifyByCitation`: citation-based classification is no longer the
  source of truth and keeping a second, weaker classifier around invites drift.
  `categories_test.go` goes with it.
- `backend/seed/avtoimtihon/assignments.json` — replaced with the full
  1260-entry mapping, so every question's topic is explicit rather than derived.
- `backend/internal/fixture/fixture.go` — sample data references 4 categories;
  update the codes it uses.

## Frontend

`frontend/src/app/[locale]/(app)/practice/page.tsx` renders categories as a flat
list of buttons keyed by `code`. It keeps doing that, inside nine collapsible
sections. The section membership is a static map from code to section key in the
frontend, and the nine section labels live in `messages/{uz-Latn,uz-Cyrl,ru}.json`.

No backend, DTO or query change: sections are an editorial grouping of a fixed,
known set of 42 codes, not content. Putting them in the database would mean a
`parent_id` column, translations, sqlc regeneration and admin CRUD for nine rows
that will never change.

Sections, matching the table above: General rules and duties (1–4), Road signs
(5–11), Road markings (12–13), Signals (14–16), Driving order (17–21),
Intersections (22–26), Special sections and priority (27–32), Vehicle and
carriage (33–40), Safety and first aid (41–42).

The first section is expanded on load, the rest collapsed. Selecting a topic
inside a section behaves exactly as selecting one of the 13 does today,
including the **Hammasi** ordered walk and its resume banner.

## Russian text corrections

Independent of the taxonomy, the same comparison surfaced real defects in our
Russian. 552 questions whose Russian diverged from ptest's or tripped a
mechanical check were reviewed; ptest's text was used as a hint only, never as
truth, because it carries its own errors ("пользуеться преимушеством",
"припятствием").

The reviewed corrections land in the repo as
`backend/seed/avtoimtihon/ru_corrections.json` — a list of
`{ext_id, slot, old, new, reason, severity}` records — and ship as a second
migration over `question_translation` and `answer_translation`, applied by exact
`old` → `new` string match so a row whose text has since changed is skipped
rather than clobbered. The same file is applied to
`backend/seed/avtoimtihon/data.json` so a fresh import carries the corrections
too.

The serious class is not spelling. Several questions are **wrong in Russian in a
way that changes the answer**:

- `avtoimtihon-230` — the correct option read "при отсутствии детского
  удерживающего устройства"; the Uzbek says "qurilma o'rnatilgan bo'lsa" (when
  one *is* fitted). A Russian-language learner was being taught the inverse rule.
- `avtoimtihon-106` — "Выключить противотуманные фары" against Uzbek "yoqishi"
  (switch on).
- `avtoimtihon-56` — the stem asked about turning right where the Uzbek asks
  about movement, and the correct option listed two cars where the Uzbek lists
  three.
- `avtoimtihon-131` — first aid: "Перевязать неповрежденную ногу" against
  "lat yegan oyoq" (the injured leg).
- `avtoimtihon-451`, `avtoimtihon-517` — the correct option lost the qualifier
  that distinguishes it from a distractor.
- `avtoimtihon-1072` — the stem was a different question entirely from its own
  answer options, which are all in the dative; unanswerable as written.
- `avtoimtihon-1127`, `avtoimtihon-1053`, `avtoimtihon-1255` — machine-translation
  damage: a stem truncated to an ellipsis, "the road on which he is *receiving
  treatment*", an invented subject.

Corrections that alter a question stem or a correct answer's text are listed for
review before the migration is written; the rest apply as reviewed.

Two items are flagged, not corrected, because they need the source or the
picture: `avtoimtihon-638` ans2 (300 m where Uzbek and ptest both say 100 m) and
`avtoimtihon-674` ans1 (which car yields to which).

## Testing

- Migration test on a seeded database: 42 categories exist with 3 translations
  each; `SELECT count(*) FROM question GROUP BY category_id` sums to 1260 and
  matches the table above topic for topic; zero questions point at a deleted
  category; the migration is idempotent under re-run and its `down` restores the
  13 with every question back in its original category (the down migration
  carries the old assignment as a literal list — it cannot be derived).
- `practice_cursor` and `category_mastery` are empty afterwards, and a fresh
  ordered walk on a new topic starts at 0 and advances.
- An existing `exam_session` row that pointed at an old category survives with
  `category_id IS NULL` and still renders in history.
- Existing content tests that assume 13 categories are updated, not deleted.
- Frontend: the picker renders nine sections; a topic inside a collapsed section
  is reachable; selecting one starts a session with the right `category_id`.
- Russian corrections: every `old` string matches exactly one row before the
  update, and no `answer.is_correct` value changes.

## Rollback

The down migration restores the 13 categories and the original per-question
assignment from a literal list. Cursors and mastery do not come back — they are
gone from the moment the up migration runs, which is the main reason to deploy
this once and not iterate on it in production.
