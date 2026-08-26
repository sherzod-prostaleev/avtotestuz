# Ordered category practice, resumed where the class left off

2026-08-27

## The problem

A teacher picks one of the 13 topics under Practice → Category, presses
**Hammasi**, and starts. Today that draws the topic's questions in random order
every time, so:

- the class never sees the material in a teachable order;
- coming back tomorrow re-draws at random, so questions repeat while others are
  never reached;
- there is no notion of "we got to 123, continue from there".

For `road_signs_markings` that is 337 questions the class can never work
through systematically.

## What changes, and what deliberately does not

**Changes:** Practice → Category → **Hammasi**. That combination becomes an
ordered walk through the topic, resumed where it stopped.

**Does not change:** the 20 / 50 / 100 presets and the manual count stay
random, exactly as today. Signs, ticket ranges and the with-image selector are
untouched. Exams, mistakes and placement are untouched.

The rule is: *ordered* is a property of "all questions of one topic", nothing
else.

## The order

`question.source_ext_id` is `avtoimtihon-<N>`. Verified against production:
all 1260 rows match `^avtoimtihon-[0-9]+$`, and all 1260 numbers are distinct.
That makes the numeric suffix a **total, unique ordering** over the whole bank
— stable no matter what else changes in the database, and the same order the
source material itself numbers the questions in.

Within a topic the numbers are not contiguous (a category's questions are
scattered through 1..1260); ascending order is what matters, not the gaps.

Sorting is `NULLIF(regexp_replace(source_ext_id, '\D', '', 'g'), '')::int`
with `source_ext_id` then `id` as tiebreakers, so a future id without digits
sorts last instead of erroring. At 1260 rows this sort costs under a
millisecond and needs no new index.

## Position

New table:

```sql
practice_cursor (
  profile_id  uuid    references profile(id) on delete cascade,
  category_id uuid    references category(id) on delete cascade,
  next_index  int     not null default 0,   -- 0-based: how many worked through
  updated_at  timestamptz not null default now(),
  primary key (profile_id, category_id)
)
```

Per profile, per topic. On a classroom PC the profile is *that station's* shadow
profile, so a 43-PC room keeps 43 independent positions: each seat resumes where
that seat stopped.

This is per-seat, not per-room, and that is the right shape even though it is
not what an early draft of this document claimed. A single shared position
across 43 machines would be actively broken — every student's answers would drag
every other student's next question forward, and nobody could work at their own
pace. Per-seat is also what a classroom actually does: students return to the
same machine, and a topic walked at one desk continues at that desk. What the
system cannot express is a teacher's "we as a class got to 123", and it should
not pretend to: a station has one profile per machine and no way to tell one
student from another.

`exam_session` gains `ordered_from int` (null for every session that is not an
ordered walk), recording the index the session started at.

## Flow

**Start.** `mode=practice`, one category selector, and `ordered: true`:

1. Read the cursor (0 if absent).
2. If it is at or past the topic's question count, reset it to 0 — this is the
   wrap: finishing a topic starts it again.
3. Draw `count` questions from the topic's ordered list starting at the cursor.
   `count` is the daily-allowance-clamped count, so a VIP or station gets the
   whole remainder and a free learner gets their 30 — in order, continuing
   tomorrow.
4. Store `ordered_from = cursor` on the session.

**Advance.** When an answer is recorded in a session with `ordered_from` set,
the cursor moves to `max(cursor, ordered_from + p)`, where `p` is the
**contiguous run of positions answered from the start of that session**.

Advancing on **answer**, not on session creation, is the property that makes
"continue from 123" true: a class that starts a 337-question session, answers
123 and closes the browser resumes at 124, not at 337. `max` keeps it
monotonic, so answering out of order inside a session never rewinds it.

Two details in that sentence are load-bearing, and both were found by audit
rather than by design:

*The contiguous run, not the furthest question touched.* The session screen
renders one clickable chip per question with no gating, so a student can scroll
to the end of a 337-question walk and answer the last one. Counted as "the
furthest position answered", that single click reads as a finished topic: the
next draw wraps, the stored position is discarded, and the class's real place is
gone with nothing on screen to recover it. The everyday version is quieter and
worse — any forward jump silently buries the questions it skipped. Production
already shows this shape: 30 of 504 answered practice sessions have a gap
between the highest position answered and the number of answers.

*Answers from an earlier walk are ignored.* The advance applies only when the
session's `ordered_from` is not ahead of the stored cursor. Practice sessions
are left open routinely — production holds 251, the oldest a month old — and the
session history makes them reopenable, so without this one answer in a stale
session would write that old walk's position over today's. A session of the
current walk always satisfies the test, because `ordered_from` is the cursor as
it stood when the session was drawn and the cursor only moves forward from
there.

**Reset.** "Boshidan boshlash" sets the cursor to 0. Needed because a new
group of students starts the topic fresh while the previous group's position
is still stored.

## API

Additive only; every existing client keeps working unchanged.

**`POST /api/v1/sessions`** — new optional request field `ordered` (bool).
Honoured only when `mode == "practice"` and `category_id` is set; ignored
otherwise rather than rejected, so an older client that sends it cannot break.
Response shape is unchanged.

**`GET /api/v1/me/practice-progress`** — the caller's position in every topic
it has started:

```json
{"data": [{"category_id": "...", "next_index": 123, "total": 337}]}
```

One call covers all 13 topics, so the practice screen can label each one
without a request per topic. Topics never started are omitted (the client
treats a missing entry as 0).

**`POST /api/v1/me/practice-progress/reset`** — body `{"category_id": "..."}`,
answers `{"data": {"reset": true}}`. Resetting a topic that was never started
succeeds; there is nothing to report and nothing to fail.

Both are per-profile, so they sit under `me/` beside `me/stats` and
`me/mistakes`, and both reach the browser through the existing
`/api/proxy/[...path]` catch-all — no new BFF routes.

## What the teacher sees

Under the **Hammasi** button, when a topic is selected and its cursor is not 0:

> 124-savoldan davom etadi · 214 ta qoldi

and a **Boshidan boshlash** control next to it. When the cursor is 0 the hint
is absent — there is nothing to resume, and an empty hint is noise.

## Testing

The order and the position are the two things that must never be wrong, so
each gets tests that fail loudly if it regresses:

- The same topic drawn twice returns the identical id sequence, and that
  sequence is ascending by source number.
- A session of the whole topic, answered partway, leaves the cursor at exactly
  the count answered; the next draw begins at that question and contains the
  remainder.
- Answering out of order inside a session never moves the cursor backwards, and
  never moves it over material that was skipped: jumping ahead holds the
  position, and filling the gap in afterwards releases it.
- Answering the last question of a long walk is one question done, not a
  finished topic.
- An answer recorded in a session from an earlier walk (before a wrap or a
  reset) leaves the current position alone.
- Abandoning a session without answering leaves the cursor untouched.
- Reaching the end wraps to 0, and the next draw is the topic from the start.
- **The second lap advances like the first.** The wrap must be persisted, not
  computed at draw time: the cursor only moves forward (the advance uses
  `GREATEST`, which is what stops an out-of-order answer rewinding a class), so
  a wrap that lived only in memory would leave 337 stored while the draw began
  at 0. Answering the first question would then write `GREATEST(337, 1) = 337`,
  the cursor would never move again, and the class would repeat the same
  questions for the life of the topic. Testing only "the draw after the end
  starts at question 1" does not catch this — the second lap has to be walked.
- Reset returns the topic to 0.
- 20 / 50 / 100 / manual, signs, ticket ranges and image-presence draws are
  still random and still ignore the cursor — a guard against the ordered path
  leaking into the rest of practice.
- All 13 topics, not just the 337-question one: every category draws its full
  count in order with no duplicates and no omissions.

## Rejected alternatives

**Resume one long session.** Create a 337-question session and reopen it on
return. Simpler, and it needs no cursor — but the daily allowance clamps a free
learner's session to 30 questions, so the session stops being "the topic" and
nothing carries the position from one session to the next. It also leaves a
session row open indefinitely and needs a separate answer for what "start over"
means.

**Seeded shuffle.** Order the topic by a hash of (topic, seed) so it is stable
per topic. Gives a repeatable order without a cursor, but answers only half the
requirement: there is still no "continue from 123". It also produces an order
with no meaning to a teacher, where the source numbering already has one.
