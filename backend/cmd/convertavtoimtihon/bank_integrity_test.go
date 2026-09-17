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
