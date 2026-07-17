// Command genfixture writes the [NAMUNA] sample dataset to a directory
// in canonical import format (data.json + images/).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"avtotest.uz/backend/internal/fixture"
)

func main() {
	out := flag.String("out", "seed/sample", "output directory")
	flag.Parse()

	ds, images := fixture.Sample()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		panic(err)
	}
	data, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "data.json"), data, 0o644); err != nil {
		panic(err)
	}
	for rel, bytes := range images {
		p := filepath.Join(*out, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			panic(err)
		}
		if err := os.WriteFile(p, bytes, 0o644); err != nil {
			panic(err)
		}
	}
	fmt.Printf("wrote %s (questions=%d variants=%d images=%d)\n",
		*out, len(ds.Questions), len(ds.Variants), len(images))
}
