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
