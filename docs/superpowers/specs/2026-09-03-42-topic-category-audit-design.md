# 42-topic category re-audit

2026-09-03

## The problem

`0069_42_topic_taxonomy.up.sql` re-pointed all 1265 questions at one of 42
YHQ-chapter topics. For 1209 of them the topic was inherited automatically
from whichever ptest.uz question paired 1:1 by text (see
`2026-08-28-42-topic-taxonomy-design.md`). That inheritance trusted ptest's
own tagging — and ptest's own tagging has confirmed misses: while auditing
today's ptest content diff, `avtoimtihon-227` ("does the driver have the
right to overtake on the right in this situation") is filed under
`lighting_devices`, which is nonsensical for that question. Nobody has since
read all 1273 questions (1265 + the 8 added today) against their actual topic
meaning. That is the gap this project closes: **re-verify every question's
`category` field by reading the question, not by trusting an inherited tag.**

## What changes, and what deliberately does not

**Changes:** `question.category` (via `backend/seed/avtoimtihon/data.json`
and the mirrored `assignments.json`) is corrected wherever it is wrong.
Nothing else. The 42 category rows themselves, their names, their
`sort_order`, and the 9-section frontend grouping are untouched — this is a
per-question fix, not another taxonomy redesign.

**Does not change:** no new questions are imported from any of the three
reference sites in this pass (that is separate, ongoing work — see
`raqobatchi-savol-monitoring-bazasi` memory). No schema change. No change to
how exams, tickets, or signs practice select questions — only the ordered
per-topic practice walk (`practice_cursor`) and per-topic mastery
(`category_mastery`) are indirectly affected, because both are keyed on
`category_id` and ordinal position within it (see Risk section).

**Ground truth is the real YHQ law**, not the reference sites. The three
sites are cross-checking signals that narrow the search and catch obvious
misses fast — the yolharakatiqoidalari.uz corpus is trusted more than the
others because it carries an actual legal citation per question, not because
majority-of-three wins automatically. Where a question's true topic is clear
from reading it and its citation, that wins even against 2-1 site agreement.

## Reference sources

| Source | Reach | What it gives us | Access |
|---|---|---|---|
| **yolharakatiqoidalari.uz** | 1264 questions | `topic` 1–42 **in the same order as our `sort_order`** (verified: all 42 bucket sizes match the live site's per-topic counts in sequence) + `correct_ans_alls`, an actual YHQ article/appendix citation per question (1260/1264 non-empty) | Local files, no scraping: `/home/sher/Рабочий стол/avtodrom/data/questions_uzl.json` (uz-Latn), `questions_uzk.json` (uz-Cyrl), `questions_ru.json` (ru) — same `id`, same order across all three. This is the owner's own other project. |
| **ptest.uz** | 1309 questions | a tag from the 42-topic tag group (`f8ef030a-71ff-4b3e-919d-6943186ba898`), tag names are what our category names were originally copied from | Open API, already the subject of `ptest-42-mavzu-tahlili` memory. `GET /api/v1/public/questions/random` harvested to convergence; `GET /api/v1/tags?limit=1000&page=1` for the tag catalogue. |
| **osonprava.uz** | ~1209 questions across its 42 topics | which of *its* 42 topics a question sits in — **not a positional match to our 42**: it is missing our `officials_duties` and has an extra "Taniqli belgilar" topic instead, so mapping is by topic **name**, not by list position | No paywall. `/topics` lists topics + counts; each topic's questions render at `/topic-test/{n}` (`n` = list position on that page, not our `sort_order`), navigable by the numbered buttons without a subscription. Requires a logged-in session (the owner's own account, already used 2026-08-30). |
| **Ours** | 1273 questions | current `category` (the thing being audited) | `backend/seed/avtoimtihon/data.json` |

**yolharakatiqoidalari.uz topic → our category** is a direct index map (topic
1 = `general_rules` … topic 42 = `first_aid`), already verified 1:1 against
the live site's topic list. **ptest tag → our category** is by tag name
(near-identical strings, since our names were copied from these tags in the
original migration). **osonprava topic → our category** is by topic name with
a manual lookup table, and two rows (`officials_duties`, "Taniqli belgilar")
have no counterpart on the other side — expected, not a bug.

## Matching method

For each of our 1273 questions, find its semantic twin (if any) in each of
the three sources. This is a **read-and-understand match, not a fuzzy-text or
programmatic one** — the previous ptest comparison already proved why:
comparing Russian text alone reads 83% overlap and undercounts; comparing
Uzbek lifts it to 96%. Image hashing across sites fails outright (55% of
already-confirmed text-pairs differ by 40+ bits) because the same rule gets
redrawn on a different background. So: Uzbek-Latin text is the primary key,
Russian is the second opinion, images are never used to accept or reject a
pairing.

A cheap programmatic pass (fuzzy string similarity, as used earlier today for
ptest) is allowed **only to shortlist 2-3 candidates per source**, cutting
what a reader has to look at. It never makes the final call — a person (the
model, reading) confirms or rejects the pairing and, independently, confirms
or rejects the current topic.

## Decision rule, per question

1. Gather whatever topic signal each of the three sources gives (none, one,
   two, or three — not every question has a twin everywhere, e.g. the 8
   questions added from ptest today have no drivergo-side history to speak
   of and may be twin-less on yolharakatiqoidalari or osonprava too).
2. If every signal found agrees with our current `category` → **leave it**,
   record as confirmed, move on. This should be the fast, common case.
3. If any signal disagrees with our current `category` → read the question,
   its answers, and (if found) the yolharakatiqoidalari citation, and decide
   the true topic from what the rule actually says. yolharakatiqoidalari's
   citation carries the most weight because it names a real YHQ article; a
   2-1 majority among the sites does not automatically override a citation
   that clearly points elsewhere, and does not automatically override the
   current category either if the sources themselves look wrong (the
   `avtoimtihon-227` /`lighting_devices` case is exactly this: one source
   being confidently wrong).
4. If no signal is found anywhere → decide from reading alone (question,
   answers, category definitions) — same standard as any hand-classified
   question in the original 51.
5. Every decision — kept or changed — gets one line of stated reasoning in
   the audit output. "Because 2 of 3 sources agreed" is not sufficient
   reasoning on its own; name what the question is actually testing.

## Pipeline

**Stage 0 — build the reference sets** (once, not per-question): load the
three local/harvested datasets into `scratch/category-audit/` (gitignored,
already used by this repo's `scratch/` convention) as flat JSON: our 1273,
yolharakatiqoidalari's 1264×3 locales, ptest's full harvest with tags,
osonprava's per-topic harvest. Re-harvesting ptest and osonprava is required
even though today's session already pulled them once — that data lived in a
session-scoped scratchpad, not this repo, and won't survive into whatever
session executes this plan.

**Stage 1 — candidate shortlist** (programmatic, fast, whole corpus at
once): for each of our 1273 questions, compute the top 2-3 candidate twins
per source by text similarity (Uzbek primary, Russian secondary). Output:
one file mapping our `ext_id` → up to 9 candidate refs (≤3 per source) with
their similarity scores. This narrows what Stage 2 has to read; it decides
nothing.

**Stage 2 — read-and-decide, first pass** (the actual audit; parallel
subagents): split our 1273 questions into batches by topic — grouping by our
*current* `category` keeps each subagent's mental model of "what does this
topic mean" stable, and keeps batch sizes naturally bounded (topic sizes
range 1–75). Roughly 7-8 subagents, each covering 5-6 topics (~150-180
questions), run in parallel. Each subagent, per question: opens the
shortlisted candidates, confirms or rejects each pairing by reading, applies
the decision rule above, and writes one row: `ext_id, current_category,
proposed_category (same if unchanged), sources found, one-line reasoning,
confidence (high/medium/low)`.

**Stage 3 — independent re-audit** (repeated audit, as asked for): every row
where `proposed_category != current_category`, plus a random 10% sample of
unchanged rows as a control, goes to a *different* subagent that did not see
Stage 2's reasoning — it re-derives the topic from scratch (question,
answers, same reference sources) and either agrees or dissents. Agreement →
the change is accepted. Dissent → escalated to the main session (me) to read
personally and make the final call; every escalation and its resolution is
recorded, none are silently dropped.

**Stage 4 — consolidate:** one report — a table of every proposed change
(`ext_id`, before → after, reasoning, Stage 2/3 agreement, any escalations)
plus corpus-wide stats (how many questions move, per-topic net in/out
counts, how many had zero signal from any source). Apply the accepted
changes to `data.json` and `assignments.json` in the working tree. **Do not
commit or deploy** — the report is what the user reviews before deciding
anything about rollout.

## Risk: production progress data

`practice_cursor.next_index` is an ordinal position *within a category's
question list*, and `category_mastery` is a cached `seen`/`correct` counter
per `(profile, category)` — both keyed on `category_id`, not on individual
question IDs. Moving a question between categories does not delete or
cascade anything (the 42 category rows themselves are untouched, unlike the
0069 migration which deleted rows and cascaded on purpose) — but it does mean
a real user's saved cursor position in an affected topic may now point at a
slightly different spot in that topic's (changed-length) ordered list, and
`category_mastery` counts for the two affected topics become approximately
rather than exactly right until they next get answered fresh.

This is accepted as a known, non-destructive side effect (per user decision
2026-09-03) — the report's stats make the blast radius (how many questions
actually move) visible, and the user decides after seeing that number whether
to ship as-is or pair it with a small recompute migration for
`category_mastery` (recomputable from `session_answer` history) and/or a
cursor-clamp for the touched categories. That decision is out of scope for
this spec; it is a rollout choice made from the Stage 4 report.

## Out of scope

- Importing new questions from any of the three sites (separate, ongoing
  work).
- Touching the 13→42 taxonomy itself, category names, or `sort_order`.
- Any code change — this is a content-only pass over two seed JSON files.
- Deciding the rollout/deploy strategy (see Risk section) — that is a
  post-report decision, not part of this plan's execution.
