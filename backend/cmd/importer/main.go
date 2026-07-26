// Command importer ingests a canonical dataset directory into the database
// and blob storage, printing a validation/import report.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"avtotest.uz/backend/internal/blob"
	"avtotest.uz/backend/internal/config"
	"avtotest.uz/backend/internal/db"
	"avtotest.uz/backend/internal/importer"
)

func main() {
	dataDir := flag.String("data", "", "dataset directory (data.json + images/)")
	verified := flag.Bool("verified", false, "mark translations as verified (licensed source)")
	explanationsOnly := flag.Bool("explanations-only", false, "refresh explanations/legal_refs only (no question rewrite)")
	flag.Parse()
	if *dataDir == "" {
		fmt.Fprintln(os.Stderr, "usage: importer -data <dir> [-verified] [-explanations-only]")
		os.Exit(2)
	}

	cfg, err := config.Load()
	fatal(err)
	raw, err := os.ReadFile(filepath.Join(*dataDir, "data.json"))
	fatal(err)
	var ds importer.Dataset
	fatal(json.Unmarshal(raw, &ds))

	fatal(db.Migrate(cfg.DatabaseURL))
	pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
	fatal(err)
	defer pool.Close()

	endpoint := getenv("MINIO_ENDPOINT", "localhost:9000")
	access := getenv("MINIO_ACCESS_KEY", "avtotest")
	secret := getenv("MINIO_SECRET_KEY", "avtotest123")
	bucket := getenv("MINIO_BUCKET", "media")
	blobs, err := blob.NewS3(endpoint, access, secret, bucket, false)
	fatal(err)

	rep, err := importer.Store(context.Background(), pool, blobs, ds, importer.StoreOptions{
		MarkVerified:     *verified,
		Images:           importer.DirSource(*dataDir),
		Source:           filepath.Base(*dataDir),
		ExplanationsOnly: *explanationsOnly,
	})
	fmt.Print(rep.String())
	fatal(err)
	if !*explanationsOnly && (rep.QuestionsQuarantined > 0 || rep.VariantsSkipped > 0) {
		fmt.Println("WARNING: quarantined/skipped entries present — review issues above")
	}
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
