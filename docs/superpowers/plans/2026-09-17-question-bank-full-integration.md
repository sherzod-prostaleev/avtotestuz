# Yangi savollarni barcha bo'limlarga to'liq kiritish va avvalgi 14 tasini tiklash — reja

> **For agentic workers:** This plan is executed **inline by the main session**
> (`superpowers:executing-plans`), NOT by subagents. Every content verdict —
> reading an image, choosing a topic, proofreading a translation, picking the
> YHQ article an explanation quotes — is made by the main session itself. The
> user's standing instruction for bank audits: *"kodga ham, subagentga ham
> ishonmasdan"*, and for this task: *"umuman chalkashlik xatolik bo'lmasligi
> juda zarur"*. Subagents may only run mechanical commands. Steps use checkbox
> (`- [ ]`) syntax for tracking.

**Goal:** Add the three ptest questions our bank does not have
(`avtoimtihon-1279..1281`) so that each one reaches every section a question
lives in — topic, bilet, sign catalogue, explanation, image, answer shuffling —
and bring the fourteen questions added earlier (`avtoimtihon-1265..1278`) to the
same standard, with permanent tests that stop the next addition from skipping a
section.

**Architecture:** Every content decision is written to an append-only JSONL/JSON
file under `scratch/qbank-2026-09-17/` the moment it is made. One script,
`apply.py`, rebuilds the three seed files as a pure function of the baseline
commit `ae1cdca` plus those decision files — so re-running it is safe and
`verify_intent.py` can prove the committed files contain the decisions and
nothing else. Go tests pin the bank-level invariants in CI.

**Tech Stack:** Python 3 stdlib + Pillow (decision/apply scripts), Go
(`/home/sher/.local/go/bin/go`, not on PATH) for tests, `golangci-lint` v2.12.2
(`~/go/bin`), Node 24 (`frontend` vitest + Playwright), `curl` for lex.uz and the
ptest public API, Docker Compose for an optional local import.

## Global Constraints

- **Scope:** `avtoimtihon-1265..1278` (the 13 ptest + 1 osonprava additions) and
  the new `avtoimtihon-1279..1281`. The 45 original-export questions without an
  explanation (`-1081`, `-1209`, `-1211`, `-1223..1264`) are OUT of scope: they
  are frozen as known debt in a test allowlist and reported to the user.
- **New questions:** `avtoimtihon-1279` = ptest `c7abec28-eea9-…` (reversing on a
  road with sign 4.1.1), `-1280` = ptest `f8a2706d-…` (unsigned T-junction, who
  goes first), `-1281` = ptest `c14973cd-…` (unsigned Y-junction, who goes last).
  `"source": "ptest"`. All three go into bilet 64 (the filling bilet, 14 → 17).
- **Never reorder answers of an already shipped question** (`1265..1278`): the
  importer upserts answer rows by `(question, position)`, so moving text between
  positions rewrites learners' answer history. Text fixes only.
- **Seed file formats (byte-exact round trip, verified):**
  `data.json` = `json.dumps(obj, ensure_ascii=False, indent=1) + "\n"`;
  `question_signs.json` = `json.dumps(obj, ensure_ascii=False, indent=2, sort_keys=True) + "\n"`;
  `assignments.json` = `json.dumps(obj, ensure_ascii=False, indent=2)` (no trailing
  newline, keys in numeric ext_id order).
- **Object key order:** question `ext_id, category, image?, texts, answers, source`;
  answer `position, correct, texts`; every locale map `ru, uz-Cyrl, uz-Latn`;
  explanation `question, legal_refs, blocks`.
- **Bank text style:** uz-Latn apostrophe is ASCII `'` (5220 uses vs 114 others);
  quotes `«»`; a question ends in `?` (or `:` for a completion stem); decimals use a
  comma in all three scripts of the same question; uz-Cyrl follows the 1995
  orthography (`автомобиль`, not `автомобил`).
- **Sign linking rule (verbatim from the 2026-09-06 audit):** link every road sign
  legibly drawn in the picture — including signs shown as numbered answer
  options — plus every sign named in the question or answer text by code or
  official name. Never link road markings, traffic lights, identification
  stickers. Only the 285 codes in `backend/cmd/gensigns/signs.go` may appear.
- **Explanation format:** exactly one `{"type": "muhim", "text": …}` block per
  locale. The text quotes the governing rule verbatim, prefixed the way the bank
  does (`YHQ 9-bobi 63-bandi: …` / `ЙҲҚ 9-боби 63-банди: …` /
  `Пункт 63 главы 9 ПДД гласит: …`), optionally followed by one sentence that
  applies it to the question. Rule text may come ONLY from the shipped
  explanations, the avtodrom `correct_ans_alls` corpus, or lex.uz — never
  paraphrased from memory. `legal_refs` are computed with
  `scripts/seed/extract_legal_refs.py:extract_refs`, never typed by hand.
- **Deploy is not part of execution.** Push and prod deploy wait for the user.

## File Map

| Path | Role |
|---|---|
| `backend/cmd/convertavtoimtihon/bank_integrity_test.go` | NEW — topic/assignment parity, one bilet per question, clean `importer.Validate`, explanation coverage with frozen legacy allowlist |
| `backend/cmd/gensigns/question_signs_test.go` | NEW — every link names a bank question and a catalogue sign |
| `scripts/seed/extract_legal_refs.py` | writes `indent=1` (today it would re-indent all of `data.json`) |
| `scripts/seed/verify-committed.py` | canonical counts updated, filling-bilet rule, orphan-explanation check |
| `Makefile` | stale `1260 / 63` wording |
| `frontend/src/lib/content-counts.ts` | `OFFICIAL_QUESTION_COUNT` 1265 → 1277 |
| `backend/seed/avtoimtihon/{data,assignments,question_signs}.json` | written only by `apply.py` |
| `backend/seed/avtoimtihon/images/i1279_1.webp` … `i1281_1.webp` | gitignored, shipped by rsync |
| `scratch/qbank-2026-09-17/` (gitignored) | evidence, decision files, `apply.py`, `verify_intent.py` |

---

### Task 1: Freeze the evidence

**Files:** Create `scratch/qbank-2026-09-17/{ptest_new.json,img/}`

- [ ] **Step 1: Copy the three ptest question objects and their original photos**

```bash
S=/tmp/claude-1000/-home-sher--------------avtotest/593cd6d6-5322-46fb-9e46-0ee514527e85/scratchpad
W="/home/sher/Рабочий стол/avtotest/scratch/qbank-2026-09-17"
mkdir -p "$W/img"
python3 - "$S" "$W" <<'EOF'
import json, sys, shutil, os
S, W = sys.argv[1], sys.argv[2]
want = {"c7abec28": "avtoimtihon-1279", "f8a2706d": "avtoimtihon-1280", "c14973cd": "avtoimtihon-1281"}
out = []
for q in json.load(open(f"{S}/ptest_harvest.json")):
    if q["id"][:8] in want:
        src = f"{S}/img_cache/{os.path.basename(q['photo'])}"
        dst = f"{W}/img/{want[q['id'][:8]].split('-')[1]}_ptest.webp"
        shutil.copyfile(src, dst)
        out.append({"ext_id": want[q["id"][:8]], "ptest": q})
json.dump(sorted(out, key=lambda r: r["ext_id"]), open(f"{W}/ptest_new.json", "w"), ensure_ascii=False, indent=1)
print(len(out), "frozen")
EOF
```

Expected: `3 frozen`.

---

### Task 2: Bank integrity tests (red first)

**Files:**
- Create: `backend/cmd/convertavtoimtihon/bank_integrity_test.go`
- Create: `backend/cmd/gensigns/question_signs_test.go`

- [ ] **Step 1: Write `bank_integrity_test.go`** (legacy allowlist = the 45 ids listed in Global Constraints)

```go
package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"avtotest.uz/backend/internal/importer"
)

const (
	shippedBankPath        = "../../seed/avtoimtihon/data.json"
	shippedAssignmentsPath = "../../seed/avtoimtihon/assignments.json"
)

// legacyExplanationGaps are questions the avtoimtihon export delivered without a
// comment, so the converter emitted no explanation for them. They are frozen as
// known debt and the list may only shrink: every question added by hand since
// (avtoimtihon-1265 onward) must ship with an explanation in all three locales.
var legacyExplanationGaps = map[string]bool{
	"avtoimtihon-1081": true, "avtoimtihon-1209": true, "avtoimtihon-1211": true,
	"avtoimtihon-1223": true, "avtoimtihon-1224": true, "avtoimtihon-1225": true,
	"avtoimtihon-1226": true, "avtoimtihon-1227": true, "avtoimtihon-1228": true,
	"avtoimtihon-1229": true, "avtoimtihon-1230": true, "avtoimtihon-1231": true,
	"avtoimtihon-1232": true, "avtoimtihon-1233": true, "avtoimtihon-1234": true,
	"avtoimtihon-1235": true, "avtoimtihon-1236": true, "avtoimtihon-1237": true,
	"avtoimtihon-1238": true, "avtoimtihon-1239": true, "avtoimtihon-1240": true,
	"avtoimtihon-1241": true, "avtoimtihon-1242": true, "avtoimtihon-1243": true,
	"avtoimtihon-1244": true, "avtoimtihon-1245": true, "avtoimtihon-1246": true,
	"avtoimtihon-1247": true, "avtoimtihon-1248": true, "avtoimtihon-1249": true,
	"avtoimtihon-1250": true, "avtoimtihon-1251": true, "avtoimtihon-1252": true,
	"avtoimtihon-1253": true, "avtoimtihon-1254": true, "avtoimtihon-1255": true,
	"avtoimtihon-1256": true, "avtoimtihon-1257": true, "avtoimtihon-1258": true,
	"avtoimtihon-1259": true, "avtoimtihon-1260": true, "avtoimtihon-1261": true,
	"avtoimtihon-1262": true, "avtoimtihon-1263": true, "avtoimtihon-1264": true,
}

// The bank files are committed, so a read failure is a broken checkout, not a
// reason to skip: an integrity gate that skips itself protects nothing.
func loadShippedBank(t *testing.T) importer.Dataset {
	t.Helper()
	raw, err := os.ReadFile(shippedBankPath)
	if err != nil {
		t.Fatalf("read %s: %v", shippedBankPath, err)
	}
	var ds importer.Dataset
	if err := json.Unmarshal(raw, &ds); err != nil {
		t.Fatalf("parse %s: %v", shippedBankPath, err)
	}
	return ds
}

// data.json and assignments.json both record a question's topic. A hand-added
// question that updates one file and not the other is filed under different
// topics depending on which file a tool reads.
func TestShippedBankTopicsMatchAssignments(t *testing.T) {
	ds := loadShippedBank(t)
	raw, err := os.ReadFile(shippedAssignmentsPath)
	if err != nil {
		t.Fatalf("read %s: %v", shippedAssignmentsPath, err)
	}
	var assignments map[string]string
	if err := json.Unmarshal(raw, &assignments); err != nil {
		t.Fatalf("parse %s: %v", shippedAssignmentsPath, err)
	}
	if len(assignments) != len(ds.Questions) {
		t.Errorf("assignments.json names %d questions, the bank has %d", len(assignments), len(ds.Questions))
	}
	for _, q := range ds.Questions {
		if want := assignments[q.ExtID]; q.Category != want {
			t.Errorf("%s: data.json topic %q, assignments.json topic %q", q.ExtID, q.Category, want)
		}
	}
}

// Most learners meet a question through a bilet: one in no bilet is reachable
// only through topic practice, and one listed twice skews a mock exam.
func TestShippedBankQuestionsSitInExactlyOneBilet(t *testing.T) {
	ds := loadShippedBank(t)
	seen := map[string]int{}
	for _, v := range ds.Variants {
		for _, ext := range v.Questions {
			seen[ext]++
		}
	}
	for _, q := range ds.Questions {
		if n := seen[q.ExtID]; n != 1 {
			t.Errorf("%s is in %d bilets, want exactly 1", q.ExtID, n)
		}
	}
}

// The importer quarantines whatever Validate flags, so a clean report is what
// "every question in the file reaches learners" means.
func TestShippedBankValidatesClean(t *testing.T) {
	ds := loadShippedBank(t)
	for _, issue := range importer.Validate(ds) {
		t.Errorf("%s %s: %s %s", issue.Entity, issue.ID, issue.Code, issue.Detail)
	}
}

// The explanation dialog and the Telegram quiz both read these rows; a question
// without one silently loses its «Izoh» button.
func TestShippedBankExplanations(t *testing.T) {
	ds := loadShippedBank(t)
	inBank := map[string]bool{}
	for _, q := range ds.Questions {
		inBank[q.ExtID] = true
	}
	explained := map[string]bool{}
	for _, e := range ds.Explanations {
		if !inBank[e.Question] {
			t.Errorf("explanation for %s, which is not in the bank", e.Question)
			continue
		}
		if explained[e.Question] {
			t.Errorf("%s has more than one explanation", e.Question)
		}
		explained[e.Question] = true
		for _, loc := range importer.RequiredLocales {
			blocks := e.Blocks[loc]
			if len(blocks) == 0 {
				t.Errorf("%s: explanation has no %s block", e.Question, loc)
				continue
			}
			for i, b := range blocks {
				text, _ := b["text"].(string)
				if b["type"] != "muhim" || strings.TrimSpace(text) == "" {
					t.Errorf("%s: %s block %d must be a non-empty muhim text block, got %v", e.Question, loc, i, b)
				}
			}
		}
	}
	for _, q := range ds.Questions {
		switch {
		case !explained[q.ExtID] && !legacyExplanationGaps[q.ExtID]:
			t.Errorf("%s ships without an explanation", q.ExtID)
		case explained[q.ExtID] && legacyExplanationGaps[q.ExtID]:
			t.Errorf("%s has an explanation now: remove it from legacyExplanationGaps", q.ExtID)
		}
	}
	for ext := range legacyExplanationGaps {
		if !inBank[ext] {
			t.Errorf("legacyExplanationGaps names %s, which is not in the bank", ext)
		}
	}
}
```

- [ ] **Step 2: Write `question_signs_test.go`**

```go
package main

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

// cmd/linkquestionsigns learns about an unknown code or a missing question only
// at deploy time, against the live database. Checking the committed map against
// the catalogue in this package and the committed bank moves that failure to CI.
func TestShippedQuestionSignLinks(t *testing.T) {
	raw, err := os.ReadFile("../../seed/avtoimtihon/question_signs.json")
	if err != nil {
		t.Fatalf("read question_signs.json: %v", err)
	}
	var links map[string][]string
	if err := json.Unmarshal(raw, &links); err != nil {
		t.Fatalf("parse question_signs.json: %v", err)
	}
	bankRaw, err := os.ReadFile("../../seed/avtoimtihon/data.json")
	if err != nil {
		t.Fatalf("read data.json: %v", err)
	}
	var bank struct {
		Questions []struct {
			ExtID string `json:"ext_id"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(bankRaw, &bank); err != nil {
		t.Fatalf("parse data.json: %v", err)
	}
	inBank := map[string]bool{}
	for _, q := range bank.Questions {
		inBank[q.ExtID] = true
	}
	catalogue := map[string]bool{}
	for _, s := range signs {
		catalogue[s.Code] = true
	}

	for ext, codes := range links {
		if !inBank[ext] {
			t.Errorf("%s is linked to signs but is not in the bank", ext)
		}
		if len(codes) == 0 {
			t.Errorf("%s has an empty sign list; drop the key instead", ext)
		}
		if !sort.StringsAreSorted(codes) {
			t.Errorf("%s: codes are not sorted: %v", ext, codes)
		}
		dup := map[string]bool{}
		for _, code := range codes {
			if !catalogue[code] {
				t.Errorf("%s links %q, which is not in the sign catalogue", ext, code)
			}
			if dup[code] {
				t.Errorf("%s links %q twice", ext, code)
			}
			dup[code] = true
		}
	}
}
```

- [ ] **Step 3: Run them — expect exactly the known gaps**

Run: `cd backend && PATH="/home/sher/.local/go/bin:$PATH" go test ./cmd/convertavtoimtihon/ ./cmd/gensigns/ -run 'TestShipped' -count=1`
Expected: FAIL only in `TestShippedBankExplanations` — 4 × "explanation for avtoimtihon-851/898/950/1020, which is not in the bank" and 14 × "avtoimtihon-1265..1278 ships without an explanation". Every other `TestShipped*` passes.

---

### Task 3: Seed tooling that silently drifted

**Files:** Modify `scripts/seed/extract_legal_refs.py`, `scripts/seed/verify-committed.py`, `Makefile`

- [ ] **Step 1: `extract_legal_refs.py` must keep the committed format** — in `main()` replace
`json.dumps(data, ensure_ascii=False, indent=2)` with
`json.dumps(data, ensure_ascii=False, indent=1)`. (Re-extraction on today's data changes 0 legal_refs, so after this fix the target is a no-op.)
- [ ] **Step 2: `verify-committed.py`** — docstring numbers updated to match, and:

```python
EXPECT = {
    "questions": 1277,
    "variants": 64,
    "explanations": 1232,  # 1215 original + 17 hand-added; the 4 orphans are gone
    "sign_groups": 7,
    "signs": 285,
    "categories": 42,
}
```

replace the every-variant-is-20 loop with the importer's rule
(`internal/importer/validate.go`: only the highest-numbered bilet may be filling):

```python
    assigned: set[str] = set()
    last = max((variant.get("number") or 0) for variant in v) if v else 0
    for variant in v:
        qs = variant.get("questions") or []
        filling = variant.get("number") == last and 1 <= len(qs) < 20
        if len(qs) != 20 and not filling:
            fail(f"variant {variant.get('number')}: {len(qs)} questions "
                 f"(want 20; only the last bilet may still be filling)")
        assigned.update(qs)
```

and, right after the variant orphan check:

```python
    bank_ids = {item.get("ext_id") for item in q}
    stray = [item.get("question") for item in e if item.get("question") not in bank_ids]
    if stray:
        fail(f"{len(stray)} explanations point at questions not in the bank (e.g. {stray[:3]})")
```
- [ ] **Step 3: `Makefile`** — `seed-verify` comment and `seed-dev` echo say `1277 questions, 64 bilets`.
- [ ] **Step 4:** `git diff --stat` shows only these three files.

---

### Task 4: Decision tooling

**Files:** Create `scratch/qbank-2026-09-17/apply.py`, `scratch/qbank-2026-09-17/verify_intent.py`

- [ ] **Step 1: Write `apply.py`** — rebuilds the seed files from `ae1cdca` + decisions:

```python
#!/usr/bin/env python3
"""Rebuild the avtoimtihon seed files from the baseline commit plus reviewed decisions.

usage: apply.py [--check]   (--check: build in memory, exit 1 if disk differs)
"""
import importlib.util
import json
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path("/home/sher/Рабочий стол/avtotest")
SEED_REL = "backend/seed/avtoimtihon"
SEED = ROOT / SEED_REL
WORK = ROOT / "scratch/qbank-2026-09-17"
BASE = "ae1cdca"
LOCALES = ("ru", "uz-Cyrl", "uz-Latn")

spec = importlib.util.spec_from_file_location("elr", ROOT / "scripts/seed/extract_legal_refs.py")
elr = importlib.util.module_from_spec(spec)
spec.loader.exec_module(elr)


def base_json(name):
    out = subprocess.run(["git", "show", f"{BASE}:{SEED_REL}/{name}"], cwd=ROOT,
                         capture_output=True, check=True).stdout
    return json.loads(out)


def jsonl(name):
    p = WORK / name
    if not p.exists():
        return []
    return [json.loads(line) for line in p.read_text(encoding="utf-8").splitlines() if line.strip()]


def build():
    data = base_json("data.json")
    assign = base_json("assignments.json")
    links = base_json("question_signs.json")
    qs = {q["ext_id"]: q for q in data["questions"]}

    new_path = WORK / "new_questions.json"
    for q in (json.loads(new_path.read_text(encoding="utf-8")) if new_path.exists() else []):
        assert q["ext_id"] not in qs, q["ext_id"]
        data["questions"].append(q)
        qs[q["ext_id"]] = q
        data["variants"][-1]["questions"].append(q["ext_id"])
        assign[q["ext_id"]] = q["category"]

    for fx in jsonl("text_fixes.jsonl"):
        q = qs[fx["ext_id"]]
        if fx["field"] == "question":
            holder = q["texts"]
        else:
            pos = int(fx["field"].split(":")[1])
            holder = next(a for a in q["answers"] if a["position"] == pos)["texts"]
        if holder[fx["locale"]] != fx["before"]:
            sys.exit(f"stale text fix, baseline differs: {fx}")
        holder[fx["locale"]] = fx["after"]

    for c in jsonl("categories.jsonl"):
        qs[c["ext_id"]]["category"] = c["category"]
        assign[c["ext_id"]] = c["category"]

    for s in jsonl("sign_links.jsonl"):
        codes = sorted(set(s["codes"]))
        if codes:
            links[s["ext_id"]] = codes
        else:
            links.pop(s["ext_id"], None)

    explanations = [e for e in data["explanations"] if e["question"] in qs]
    by_q = {e["question"]: e for e in explanations}
    for r in jsonl("explanations.jsonl"):
        blocks = {loc: [{"type": "muhim", "text": r["blocks"][loc]}] for loc in LOCALES}
        e = by_q.get(r["ext_id"])
        if e is None:
            e = {"question": r["ext_id"], "legal_refs": [], "blocks": blocks}
            explanations.append(e)
            by_q[r["ext_id"]] = e
        e["blocks"] = blocks
        e["legal_refs"] = elr.extract_refs(elr.blocks_text(blocks))
    data["explanations"] = explanations

    return {
        "data.json": json.dumps(data, ensure_ascii=False, indent=1) + "\n",
        "assignments.json": json.dumps(assign, ensure_ascii=False, indent=2),
        "question_signs.json": json.dumps(links, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
    }


def main():
    files = build()
    images = {f"i{n}_1.webp": WORK / "img" / f"{n}_ptest.webp" for n in (1279, 1280, 1281)}
    if "--check" in sys.argv:
        bad = [n for n, text in files.items() if (SEED / n).read_text(encoding="utf-8") != text]
        bad += [n for n, src in images.items()
                if src.exists() and (SEED / "images" / n).read_bytes() != src.read_bytes()]
        print("MISMATCH:" if bad else "OK: seed files == baseline + decisions", *bad)
        sys.exit(1 if bad else 0)
    for n, text in files.items():
        (SEED / n).write_text(text, encoding="utf-8")
    for n, src in images.items():
        if src.exists():
            shutil.copyfile(src, SEED / "images" / n)
    print("written")


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Write `verify_intent.py`** — prints, for each of `1265..1281`, the final three-locale question + answers (✔ marks), topic, bilet, sign links, image size, explanation text and legal_refs, and asserts: only those ext_ids differ from `ae1cdca` (plus the 4 removed orphan explanations); answers of `1265..1278` keep their baseline positions and correct flags; `apply.py --check` passes.

```python
#!/usr/bin/env python3
"""Show the final state of every touched question and prove nothing else moved."""
import json
import subprocess
import sys
from pathlib import Path

ROOT = Path("/home/sher/Рабочий стол/avtotest")
SEED = ROOT / "backend/seed/avtoimtihon"
BASE = "ae1cdca"
TOUCHED = {f"avtoimtihon-{n}" for n in range(1265, 1282)}
ORPHANS = {"avtoimtihon-851", "avtoimtihon-898", "avtoimtihon-950", "avtoimtihon-1020"}


def base(name):
    return json.loads(subprocess.run(["git", "show", f"{BASE}:backend/seed/avtoimtihon/{name}"],
                                     cwd=ROOT, capture_output=True, check=True).stdout)


def main():
    b, d = base("data.json"), json.loads((SEED / "data.json").read_text(encoding="utf-8"))
    ba, a = base("assignments.json"), json.loads((SEED / "assignments.json").read_text(encoding="utf-8"))
    bl, l = base("question_signs.json"), json.loads((SEED / "question_signs.json").read_text(encoding="utf-8"))
    errors = []

    bq = {q["ext_id"]: q for q in b["questions"]}
    dq = {q["ext_id"]: q for q in d["questions"]}
    for k in set(bq) | set(dq):
        if k not in TOUCHED and bq.get(k) != dq.get(k):
            errors.append(f"untouched question changed: {k}")
    for k in TOUCHED & set(bq):
        for ba_, da_ in zip(bq[k]["answers"], dq[k]["answers"]):
            if (ba_["position"], ba_["correct"]) != (da_["position"], da_["correct"]):
                errors.append(f"{k}: answer order/correctness moved")
        if len(bq[k]["answers"]) != len(dq[k]["answers"]):
            errors.append(f"{k}: answer count changed")
    be = {e["question"]: e for e in b["explanations"]}
    de = {e["question"]: e for e in d["explanations"]}
    for k in set(be) | set(de):
        if k in TOUCHED:
            continue
        if k in ORPHANS:
            if k in de:
                errors.append(f"orphan explanation still present: {k}")
        elif be.get(k) != de.get(k):
            errors.append(f"untouched explanation changed: {k}")
    for k in set(ba) | set(a):
        if k not in TOUCHED and ba.get(k) != a.get(k):
            errors.append(f"untouched assignment changed: {k}")
    for k in set(bl) | set(l):
        if k not in TOUCHED and bl.get(k) != l.get(k):
            errors.append(f"untouched sign link changed: {k}")
    if b["variants"][:-1] != d["variants"][:-1]:
        errors.append("a full bilet changed")
    bilet_of = {e: v["number"] for v in d["variants"] for e in v["questions"]}

    for n in range(1265, 1282):
        k = f"avtoimtihon-{n}"
        q = dq.get(k)
        if q is None:
            errors.append(f"{k} missing")
            continue
        print(f"\n===== {k}  topic={q['category']}  bilet={bilet_of.get(k)}  image={q.get('image')}  signs={l.get(k)}")
        for loc in ("uz-Latn", "uz-Cyrl", "ru"):
            print(f"  [{loc}] {q['texts'][loc]}")
            for ans in q["answers"]:
                print(f"     {ans['position']}{'✔' if ans['correct'] else ' '} {ans['texts'][loc]}")
        e = de.get(k)
        if e is None:
            errors.append(f"{k} has no explanation")
        else:
            print("  legal_refs:", json.dumps(e["legal_refs"], ensure_ascii=False))
            for loc in ("uz-Latn", "uz-Cyrl", "ru"):
                print(f"  izoh[{loc}] {e['blocks'][loc][0]['text']}")

    check = subprocess.run([sys.executable, str(ROOT / "scratch/qbank-2026-09-17/apply.py"), "--check"])
    if check.returncode:
        errors.append("apply.py --check failed")
    print("\n" + ("\n".join("ERROR " + x for x in errors) if errors else "INTENT VERIFIED"))
    sys.exit(1 if errors else 0)


if __name__ == "__main__":
    main()
```

- [ ] **Step 3:** `python3 scratch/qbank-2026-09-17/apply.py --check` → `OK: seed files == baseline + decisions` (no decision files yet, so the build equals `ae1cdca`).

---

### Task 5: Review and fix the texts of 1265–1278

**Files:** Create `scratch/qbank-2026-09-17/text_fixes.jsonl`

Decision line: `{"ext_id", "field": "question"|"answer:<pos>", "locale", "before", "after", "why"}` (`before` = exact baseline text).

- [ ] **Step 1:** Read every question and every answer of `1265..1278` in all three scripts side by side against the ptest originals. Known defects to confirm and fix (the read-through may add more):
  - `-1265` answer 3 uz-Latn `Yashil, sariq, qizi` → `Yashil, sariq, qizil`
  - `-1267` answers 1–3 uz-Latn `0.8 mm`/`0.6 mm`/`0.4 mm` → comma form (uz-Cyrl and ru already use a comma)
  - `-1269` question uz-Latn has no final `?`
  - `-1272` answer 1 uz-Latn ends `…sidirg'a chiziqn` → `…sidirg'a chiziqni`
  - `-1274` question: `"A"` → `«A»` (uz-Latn, uz-Cyrl); `ruxsat etildimi` → `ruxsat etiladimi` (uz-Latn), `рухсат этилдими` → `рухсат этиладими` (uz-Cyrl)
  - `-1275` ru question omits «90 km/soat» that uz states → translate the full condition; ru answer 1 `знаки; ограничивающие` → `знаки, ограничивающие`
  - `-1277` uz-Cyrl `автомобил` → `автомобиль`
- [ ] **Step 2:** Append one JSONL line per changed string; `python3 apply.py && python3 verify_intent.py` — no `stale text fix`, untouched questions unchanged.

---

### Task 6: Verify the topic of 1265–1278

**Files:** Create `scratch/qbank-2026-09-17/categories.jsonl` (only for changes)

- [ ] **Step 1:** For each question, name the governing rule (chapter/band/appendix) and the topic its siblings with the same rule sit in (`search_bank.py`). Record the verdict for all 14 in `scratch/qbank-2026-09-17/category_review.md` (keep or change, with the rule). Open questions to settle with the legal source: `-1266` (camera-recorded liability) and `-1276` (when a YPX officer may ask the driver to step out) — `officials_duties` vs `driver_duties`.
- [ ] **Step 2:** Only a change goes to `categories.jsonl` as `{"ext_id", "category", "why"}`; apply + verify.

---

### Task 7: Verify the sign links of 1265–1278

**Files:** Create `scratch/qbank-2026-09-17/sign_links.jsonl`

- [ ] **Step 1:** Images `i1269_1`, `i1273_1`, `i1274_1`, `i1277_1`, `i1278_1` opened and zoomed; texts scanned for sign codes/names. Confirmed so far: `-1274` shows 5.5 (linked) **and 5.9** (square blue, arrow over route-vehicle pictogram — not linked); the other four images show no road sign.
- [ ] **Step 2:** `{"ext_id": "avtoimtihon-1274", "codes": ["5.5", "5.9"], "evidence": "..."}`; apply + verify.

---

### Task 8: Explanations for 1265–1278 and the orphan cleanup

**Files:** Create `scratch/qbank-2026-09-17/explanations.jsonl`

Decision line: `{"ext_id", "sources": ["bank:avtoimtihon-722", "lex.uz/docs/…#art"], "blocks": {"ru": …, "uz-Cyrl": …, "uz-Latn": …}}`

- [ ] **Step 1:** For each question find the governing rule text in all three scripts with `scratch/…/yhq.py grep` (shipped explanations + avtodrom). Rules outside the YHQ corpus (`-1266` liability, `-1268` purpose of the law, `-1276` officer powers) come from lex.uz (`curl`, official text). A rule that cannot be sourced verbatim is reported to the user instead of written.
- [ ] **Step 2:** Check that the quoted rule actually yields the question's correct answer (not just the topic). Write the line.
- [ ] **Step 3:** `apply.py && verify_intent.py`; the four orphan explanations disappear by construction (`apply.py` keeps only explanations whose question is in the bank).
- [ ] **Step 4:** `cd backend && PATH="/home/sher/.local/go/bin:$PATH" go test ./cmd/convertavtoimtihon/ -run TestShippedBankExplanations -count=1` → PASS (1279–1281 are not in the bank yet).

---

### Task 9: Add avtoimtihon-1279..1281

**Files:** Create `scratch/qbank-2026-09-17/new_questions.json`; append to `sign_links.jsonl`, `explanations.jsonl`

- [ ] **Step 1: Texts** — start from `ptest_new.json`; normalize to the bank style (ASCII apostrophe, `?`, Cyrillic orthography), proofread all three scripts, keep ptest's answer order (1279: 3 answers ✔3; 1280: 4 answers ✔2; 1281: 4 answers ✔2).
- [ ] **Step 2: Picture check** — zoom each image; confirm the scenario that makes the ptest answer correct (1280/1281: bus signalling right, truck straight, car signalling left) and look for watermarks/brand text.
- [ ] **Step 3: Topic** — 1279 `starting_manoeuvring` (reversing, YHQ 9-bob 63-band; siblings `-878`, `-942`, `-1269`); 1280 and 1281 `intersections_equal` (no signs, no signals; YHQ 16-bob 105-band) — confirm in `category_review.md`.
- [ ] **Step 4: Object** — `{"ext_id", "category", "image": "images/i<N>_1.webp", "texts": {ru, uz-Cyrl, uz-Latn}, "answers": [{"position", "correct", "texts"}], "source": "ptest"}` in that key order.
- [ ] **Step 5: Signs** — 1279 → `["4.1.1"]` after zoom confirmation; 1280/1281 none unless the zoom finds one.
- [ ] **Step 6: Explanations** — as Task 8.
- [ ] **Step 7:** `apply.py && verify_intent.py`; answer-shuffle check: 1279 and 1280 shuffle, 1281 is order-locked (`Barchasi bir vaqtda` matches `\b(?:barchasi|hammasi)`) — harmless, documented.

---

### Task 10: Frontend pre-fetch fallback

**Files:** Modify `frontend/src/lib/content-counts.ts`

- [ ] **Step 1:** `export const OFFICIAL_QUESTION_COUNT = 1277;`
- [ ] **Step 2:** `cd frontend && rm -rf .next && npx vitest run && CI=true PORT=3112 npx playwright test` → all green.

---

### Task 11: Full verification

- [ ] **Step 1:** `cd backend && PATH="/home/sher/.local/go/bin:$PATH" go test ./cmd/convertavtoimtihon/ ./cmd/gensigns/ ./internal/importer/ ./internal/explanation/ -count=1` → ok
- [ ] **Step 2:** `PATH="/home/sher/.local/go/bin:$PATH" go test ./internal/session/ -run 'TestOrderLock|TestOrdered' -count=1` → ok (bank-reading tests; pin the locked share ≤ 20%)
- [ ] **Step 3:** `PATH="/home/sher/.local/go/bin:$HOME/go/bin:$PATH" golangci-lint run ./cmd/convertavtoimtihon/... ./cmd/gensigns/...` → `0 issues`
- [ ] **Step 4:** `python3 scripts/seed/verify-committed.py` → `OK committed seed parity` with the new counts; `python3 scripts/seed/extract_legal_refs.py && git diff --stat backend/seed` → no change from the extractor
- [ ] **Step 5:** `python3 scratch/qbank-2026-09-17/verify_intent.py` → `INTENT VERIFIED`; then a second complete read of its printout (all 17 questions, answers, explanations) as a fresh reviewer.
- [ ] **Step 6 (if the local stack starts):** `docker compose up -d postgres minio redis`, then `make seed-reset-content seed-import seed-signs seed-link-signs` → importer `1277 valid, 0 quarantined, 64 variants`, no `explanation_unknown_question`; linker `unknown_signs=0 missing_questions=0`; SQL spot-check of topic, bilet, `question_sign`, `explanation_translation` rows for 1265–1281.

---

### Task 12: Commit and hand over

- [ ] **Step 1:** Commits: (a) tests + tooling + frontend constant, (b) content (`data.json`, `assignments.json`, `question_signs.json`) + this plan. Messages per repo convention with the session attribution lines.
- [ ] **Step 2:** Report to the user in Uzbek: what was wrong in the 14, what was added, the 45 legacy gaps, and ask before push/deploy (deploy needs rsync of the three new images, importer with `-e MINIO_BUCKET=media` on `drivergo_default`, `linkquestionsigns`, web rebuild for the constant, smoke).
- [ ] **Step 3:** Update memories `raqobatchi-savol-monitoring-bazasi.md` / `ptest-42-mavzu-tahlili.md` with the result and the new integrity gates.
