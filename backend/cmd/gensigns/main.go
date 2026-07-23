// Command gensigns emits the canonical sign catalogue (sign groups + signs)
// as a dataset the importer can ingest. The catalogue itself lives in
// signs.go as a reviewed, version-controlled list; this command only shapes it
// into importer.Dataset JSON so nothing about the data is decided at run time.
//
//	go run ./cmd/gensigns -out seed/signs
//	go run ./cmd/importer -data seed/signs -verified
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"avtotest.uz/backend/internal/importer"
)

type groupSeed struct {
	Code     string
	Sort     int
	NameLatn string
	NameRu   string
}

type signSeed struct {
	Code     string
	Group    string
	NameLatn string
	NameRu   string
	// Image is a relative path inside the dataset dir, empty until the SVGs
	// are vendored. The importer stores a null image for an empty path.
	Image string
}

func build() (importer.Dataset, error) {
	known := map[string]bool{}
	ds := importer.Dataset{}
	for _, g := range groups {
		if known[g.Code] {
			return ds, fmt.Errorf("duplicate group code %q", g.Code)
		}
		known[g.Code] = true
		ds.SignGroups = append(ds.SignGroups, importer.CanonSignGroup{
			Code: g.Code, Sort: g.Sort,
			Names: map[string]string{"uz-Latn": g.NameLatn, "ru": g.NameRu},
		})
	}

	seenSign := map[string]bool{}
	for i, s := range signs {
		if !known[s.Group] {
			return ds, fmt.Errorf("sign %q references unknown group %q", s.Code, s.Group)
		}
		if seenSign[s.Code] {
			return ds, fmt.Errorf("duplicate sign code %q", s.Code)
		}
		seenSign[s.Code] = true
		ds.Signs = append(ds.Signs, importer.CanonSign{
			Code: s.Code, Group: s.Group, Image: s.Image, Sort: i + 1,
			Names: map[string]string{"uz-Latn": s.NameLatn, "ru": s.NameRu},
		})
	}
	// Stable output so re-runs produce identical files and diffs stay readable.
	sort.SliceStable(ds.Signs, func(a, b int) bool { return ds.Signs[a].Sort < ds.Signs[b].Sort })
	return ds, nil
}

func main() {
	out := flag.String("out", "seed/signs", "output dataset directory")
	flag.Parse()

	ds, err := build()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	raw, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(*out, "data.json"), append(raw, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d groups, %d signs to %s/data.json\n", len(ds.SignGroups), len(ds.Signs), *out)
}
