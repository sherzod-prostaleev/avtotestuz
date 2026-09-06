# Savol ↔ yo'l belgisi bog'lanishlari auditi — reja

> **For agentic workers:** This plan is executed **inline by the main session**
> (`superpowers:executing-plans`), NOT by subagents. The user's explicit
> instruction is that the sign identification must be done by the main
> session's own eyes — *"aynan kodga yoki kimgadir ishonmaysan, to'liq o'z
> ko'zing o'z kuchingga ishonasan"*. Subagents may only be used for mechanical
>, non-judgment work (never for looking at a question image and deciding which
> sign it shows). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild `backend/seed/avtoimtihon/question_signs.json` from scratch by
looking at every one of the 743 question images, so that every road sign is
linked to exactly the questions it actually appears in.

**Architecture:** Six sequential passes writing to append-only JSONL files under
`scratch/sign-audit/`. Nothing is decided in memory alone — every batch's verdict
hits disk immediately, so a context compaction loses at most one batch. The final
`question_signs.json` is *derived* from those JSONL files by a script, then
reconciled byte-for-byte against them.

**Tech Stack:** Python 3 (stdlib only), ImageMagick (`convert`/`identify`) for
crop-zoom on unreadable signs, Go 1.26 (`/home/sher/.local/go/bin/go`) for
`cmd/linkquestionsigns`, Docker Compose for the dev Postgres.

**Spec:** `docs/superpowers/specs/2026-09-06-question-sign-link-audit-design.md`

## Global Constraints

- **Linking rule (verbatim from spec):** link every road sign legibly drawn in
  the picture — including signs shown as numbered answer options (all of them,
  not only the correct one) — plus every sign named in the question or answer
  text by code (`3.24`) or official name («Avtomagistral»).
- **Never link:** identification stickers («Nogiron», «Shiplar», «Yosh
  haydovchi»), road markings, traffic lights, advertising boards, street-name
  plates.
- **Only the 285 codes in `backend/seed/signs/data.json` may appear** in the
  output. Anything else goes to `not_in_catalog` in the notes, never to the map.
- **The existing map is a claimant, not a source.** Every disagreement with it
  gets the image opened a second time (Pass 3).
- **onless.uz** may be consulted only for individual signs that cannot be
  identified from the image, sequentially — never parallel Chrome subagents.
- Output file format is unchanged: a JSON object `ext_id → sorted unique
  string[]`, 2-space indent, keys sorted the same way the current file sorts
  them (plain lexicographic on the ext_id string).
- Work directly on `main`; no feature branch (project convention).

---

### Task 0: Scaffolding and the sign-catalogue reference

**Files:**
- Create: `scratch/sign-audit/tool.py`
- Create: `scratch/sign-audit/catalog-notes.md`
- Read: `backend/seed/signs/data.json`, `backend/seed/signs/images/*.png`

**Interfaces:**
- Produces: `tool.py` with exactly these subcommands, used by every later task:
  `init`, `batch --start N --count K [--pass 1|3|text]`,
  `status [--pass 1|3|text]`, `text-candidates`, `relook-worklist`,
  `conflicts`, `zero-sign`, `per-sign`, `build --out PATH`, `diff-summary`,
  `report`, `verify`.
- Produces: `scratch/sign-audit/catalog-notes.md` — the code↔appearance crib
  sheet consulted throughout Pass 1.

- [ ] **Step 1: Create the work dir and the tool**

`scratch/` is already gitignored, so nothing here reaches the repo.

```bash
mkdir -p scratch/sign-audit
```

`tool.py` must implement exactly these subcommands (all paths relative to the
repo root):

- `batch --start N --count K` — print questions `N..N+K-1` from the *worklist*
  (the ordered list of ext_ids to review in this pass) as a compact block:
  ext_id, category, image absolute path, uz-Latn stem, ru stem, and every
  answer with `*` marking the correct one. It must also print the codes the
  Onless map currently claims for that question, clearly labelled as a claim.
- `status` — how many worklist entries already have a verdict in the pass's
  JSONL, and the index of the first one that does not.
- `conflicts` — ext_ids where the recorded verdict differs (as a set) from the
  Onless claim.
- `zero-sign` — ext_ids whose recorded verdict has an empty `codes` list.
- `build --out PATH` — derive the final map from the JSONL files, applying the
  precedence pass3 > pass2-text > pass1, dropping entries with empty `codes`,
  sorting codes, and writing JSON with 2-space indent.
- `report` — write `scratch/sign-audit/report.md`.
- `verify` — rebuild the map from the JSONL files a second time and assert it
  is byte-identical to `backend/seed/avtoimtihon/question_signs.json`.

- [ ] **Step 2: Freeze the worklists**

```bash
python3 scratch/sign-audit/tool.py init
```

Writes `scratch/sign-audit/worklist-images.json` (743 ext_ids, ordered by the
numeric part of `avtoimtihon-N`) and `scratch/sign-audit/worklist-text.json`
(the 531 without an image). Ordering must be deterministic and stable — this is
what "resume at index 240" means.

- [ ] **Step 3: Read the whole sign catalogue**

Read all 285 images under `backend/seed/signs/images/` group by group (warning
50, priority 9, prohibiting 38, mandatory 24, info 76, service 30,
supplementary 58), alongside their uz-Latn and ru names from
`backend/seed/signs/data.json`.

- [ ] **Step 4: Write the crib sheet**

`scratch/sign-audit/catalog-notes.md` must record, for every confusable family,
the one visual feature that separates its members. At minimum: `1.11.x`,
`1.12.x`, `2.3.x`, `3.27`/`3.28`, `4.1.x`, `5.8.x`, `5.16.1`/`5.16.2`,
`5.19.1`/`5.19.2`, `7.4.x`, `7.13`. Anything noticed as ambiguous while reading
goes here too.

- [ ] **Step 5: Sanity-check the tool**

```bash
python3 scratch/sign-audit/tool.py batch --start 0 --count 2
python3 scratch/sign-audit/tool.py status
```

Expected: two full question blocks with absolute image paths that exist;
`status` reports `0 / 743 done, resume at index 0`.

---

### Task 1: Pass 1 — all 743 picture questions

**Files:**
- Append: `scratch/sign-audit/pass1.jsonl`

**Interfaces:**
- Consumes: `tool.py batch`, `catalog-notes.md`.
- Produces: one JSONL line per ext_id:

```json
{"ext_id":"avtoimtihon-105","codes":["4.3"],"confidence":"high","unreadable":[],"not_in_catalog":[],"note":""}
```

`confidence` is `high` | `medium` | `low`. `unreadable` holds short prose
descriptions of signs that are visible but not identifiable. `not_in_catalog`
holds descriptions of signs identified but absent from the 285. `note` is free
text (why a claim was rejected, what the arrow points at, etc.).

- [ ] **Step 1: Review one batch of 10**

Per batch, exactly two tool rounds:

1. One `Bash` call that **appends the previous batch's verdicts and prints the
   next batch**, so no round trip is wasted:

```bash
cd "/home/sher/Рабочий стол/avtotest" && cat >> scratch/sign-audit/pass1.jsonl <<'EOF'
{"ext_id":"avtoimtihon-1","codes":["3.27","7.18"],"confidence":"high","unreadable":[],"not_in_catalog":[],"note":""}
EOF
python3 scratch/sign-audit/tool.py batch --start 10 --count 10
```

2. One block of 10 parallel `Read` calls on the printed image paths.

Judge each image against the Global Constraints rule. Decide each image's codes
*before* looking at the Onless claim printed in the block; use the claim only as
a prompt to look again, never as an answer.

- [ ] **Step 2: Zoom when a sign is too small to read**

Do not guess and do not mark `high` on a squint. Crop and enlarge instead:

```bash
convert "backend/seed/avtoimtihon/images/i11_5.webp" -crop 200x200+380+150 -resize 400% /tmp/claude-1000/-home-sher--------------avtotest/1486d9da-7fd8-48d3-ac6c-7db50e0527ab/scratchpad/zoom.png
```

Then `Read` the zoom. If it is still unreadable, put a description in
`unreadable`, leave the code out, and set `confidence` to `low`.

- [ ] **Step 3: Repeat until the worklist is exhausted**

```bash
python3 scratch/sign-audit/tool.py status
```

Expected at the end: `743 / 743 done`.

- [ ] **Step 4: Commit nothing yet**

Pass 1 output lives in `scratch/` (gitignored) on purpose — the repo gets a
single reviewed change at Task 5, not 75 intermediate ones.

---

### Task 2: Pass 2 — the 531 questions without an image

**Files:**
- Append: `scratch/sign-audit/pass2-text.jsonl`

**Interfaces:**
- Consumes: `worklist-text.json`.
- Produces: JSONL lines in the Task 1 schema plus `"source":"text"`.

- [ ] **Step 1: Shortlist mechanically, decide personally**

```bash
python3 scratch/sign-audit/tool.py text-candidates > scratch/sign-audit/text-candidates.txt
wc -l scratch/sign-audit/text-candidates.txt
```

`text-candidates` flags a question when its stem or any answer, in any of the
three locales, contains either a code matching `\b\d\.\d+(\.\d+)?\b` that is in
the catalogue, or an official sign name from `backend/seed/signs/data.json`
(case-insensitive, apostrophe-normalised `'`/`ʼ`/`'`).

- [ ] **Step 2: Read every shortlisted question in full and record a verdict**

A mention only counts under rule 2 when it names a sign — "5.15 yoki 6.11 yo'l
belgilari bilan belgilangan joyda" counts; a bare number that happens to look
like a code (a distance, a speed, a clause number of the YHQ) does not.

- [ ] **Step 3: Record an explicit empty verdict for the rest**

Every one of the 531 must end up with a line, `codes: []` where nothing is
named, so Task 6's reconciliation can prove nothing was skipped.

```bash
python3 scratch/sign-audit/tool.py status --pass text
```

Expected: `531 / 531 done`.

---

### Task 3: Pass 3 — the targeted second look

**Files:**
- Append: `scratch/sign-audit/pass3.jsonl`

**Interfaces:**
- Consumes: `pass1.jsonl`, the Onless map, `catalog-notes.md`.
- Produces: JSONL lines in the Task 1 schema. A line here **overrides** pass 1
  for that ext_id.

- [ ] **Step 1: Build the re-look worklist**

```bash
python3 scratch/sign-audit/tool.py relook-worklist
```

It must select the union of:
- (a) every ext_id where the pass-1 verdict differs as a set from the Onless claim;
- (b) every ext_id with `confidence != "high"`;
- (c) every ext_id with `codes == []`;
- (d) every ext_id whose codes include a member of a confusable family
  (`1.11.`, `1.12.`, `2.3.`, `3.27`, `3.28`, `4.1.`, `5.8.`, `5.16.`, `5.19.`,
  `7.4.`, `7.13`).

- [ ] **Step 2: Re-open every selected image**

Same two-round batch rhythm as Task 1. The pass-1 verdict is *not* printed for
these — decide from the image again, then compare. Where the second look
disagrees with the first, the second look wins, and the disagreement is recorded
in `note`.

- [ ] **Step 3: Verify the worklist was exhausted**

```bash
python3 scratch/sign-audit/tool.py status --pass 3
```

Expected: every selected ext_id has a line.

---

### Task 4: Inverse verification — per sign, not per question

**Files:**
- Create: `scratch/sign-audit/per-sign.md`

- [ ] **Step 1: Produce the per-sign view**

```bash
python3 scratch/sign-audit/tool.py per-sign > scratch/sign-audit/per-sign.md
```

For each of the 285 codes: the new question count, the old (Onless) count, and
the ext_id list.

- [ ] **Step 2: Re-examine the two failure shapes**

- **count 0** — a sign nothing links to. Confirm it genuinely appears nowhere:
  grep the Onless map for it, and if Onless claimed it, re-open those images.
- **count far above its neighbours** — a code that may have been pasted onto
  images that show a sibling. Spot-check its images until the outlier is
  explained.

Corrections found here are appended to `pass3.jsonl` (it is the override layer).

---

### Task 5: Build, validate, apply

**Files:**
- Modify: `backend/seed/avtoimtihon/question_signs.json`

- [ ] **Step 1: Build the new map**

```bash
python3 scratch/sign-audit/tool.py build --out backend/seed/avtoimtihon/question_signs.json
git diff --stat backend/seed/avtoimtihon/question_signs.json
```

- [ ] **Step 2: Prove the diff is data-only**

```bash
python3 scratch/sign-audit/tool.py diff-summary
```

Expected: a summary of added/removed/kept links, and confirmation that the file
still parses as `map[string][]string` with every code in the catalogue.

- [ ] **Step 3: Run the real linker against the dev database**

```bash
docker compose up -d --wait postgres
cd backend && PATH="/home/sher/.local/go/bin:$PATH" go run ./cmd/linkquestionsigns -links seed/avtoimtihon/question_signs.json
```

Expected: `unknown_signs=0`, `missing_questions=0`, and `linked_rows` equal to
the total link count reported by `diff-summary`.

If the dev database has no content, seed it first with `make seed-dev` — never
against production.

- [ ] **Step 4: Run the Go unit tests that touch this data**

```bash
cd backend && PATH="/home/sher/.local/go/bin:$PATH" go test ./internal/importer/... ./internal/content/... ./internal/session/...
```

Expected: PASS.

---

### Task 6: Final programmatic reconciliation and report

This is the step the 42-topic audit learned the hard way: check the **whole**
corpus against intent, not just the entries believed to have changed.

- [ ] **Step 1: Verify the committed file equals the JSONL-derived intent**

```bash
python3 scratch/sign-audit/tool.py verify
```

`verify` must rebuild the map independently from `pass1/pass2-text/pass3` and
assert byte equality with `backend/seed/avtoimtihon/question_signs.json`, and
assert that all 1274 ext_ids are accounted for (every question has a verdict in
some pass, even if empty). Any mismatch fails loudly.

- [ ] **Step 2: Write the report**

```bash
python3 scratch/sign-audit/tool.py report
```

`scratch/sign-audit/report.md` covers: link counts added/removed/kept; every
disagreement with Onless and its reason; per-sign counts old vs new;
`unreadable`; `not_in_catalog`; signs still at zero; and the out-of-scope
follow-up the spec calls for — the signs whose new question count makes
`/signs` start an impractically long practice session.

---

### Task 7: Commit and push

- [ ] **Step 1: Review the diff one final time**

```bash
git status && git diff --stat
```

Only `backend/seed/avtoimtihon/question_signs.json` should be modified.

- [ ] **Step 2: Commit and push**

```bash
git add backend/seed/avtoimtihon/question_signs.json
git commit -F - <<'EOF'
content(signs): rebuild question↔sign links from the pictures

The body states the real numbers produced by `tool.py report` — questions
reviewed, links added, links removed, signs that gained their first question —
and names two or three concrete corrections (e.g. avtoimtihon-105 was mapped to
3.19 + 5.8.1 while the picture shows 4.3).

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_013EwpnWXnJ59eocSr38J6QP
EOF
git push origin main
```

---

### Task 8: Production deploy

`question_sign` is a pure join table — nothing has a foreign key into it, and
`linkquestionsigns` does `DELETE` + `INSERT` inside one transaction. No learner
progress is touched, so this needs no migration and no importer run.

- [ ] **Step 1: Sync the repo to the VPS**

Follow `prod-deploy-protsedurasi` (the README command does not work on the
server): `deploy/sync-to-vps.sh --apply`.

- [ ] **Step 2: Re-run the linker on production**

Inside the backend container, against the production database, with
`--network drivergo_default`:

```bash
go run ./cmd/linkquestionsigns -links seed/avtoimtihon/question_signs.json
```

Expected: `unknown_signs=0 missing_questions=0`.

- [ ] **Step 3: Smoke-test**

```bash
deploy/smoke.sh
```

Expected: healthz / readyz / station-manifest / web all OK.

- [ ] **Step 4: Check the live signs page**

Open `/uz/signs` and confirm the question-count badges changed and a sign
opens a practice session whose first question really shows that sign.
