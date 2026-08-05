// Command avtotest-station runs one classroom PC: it holds the station key,
// keeps an access token live, serves the browser from localhost and opens the
// kiosk.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/hwid"
	"avtotest.uz/station/internal/keystore"
	"avtotest.uz/station/internal/kiosk"
	"avtotest.uz/station/internal/proxy"
)

// version is stamped at build time: go build -ldflags "-X main.version=1.0.0"
var version = "dev"

func main() {
	var (
		code     = flag.String("code", "", "one-time org enrollment code (first run only)")
		label    = flag.String("label", "", "PC name shown to the school (default: hostname)")
		apiBase  = flag.String("api", "https://api.avtotest.uz", "backend base URL")
		frontend = flag.String("frontend", "https://avtotest.uz", "frontend base URL")
		addr     = flag.String("addr", "127.0.0.1:17817", "local listen address")
		stateDir = flag.String("state", defaultStateDir(), "directory for the station key and state")
		noKiosk  = flag.Bool("no-kiosk", false, "serve only; do not launch a browser")
	)
	flag.Parse()

	id, err := hwid.Collect()
	if err != nil {
		log.Fatalf("hardware id: %v", err)
	}
	keys, err := keystore.Open(*stateDir)
	if err != nil {
		log.Fatalf("keystore: %v", err)
	}
	name := *label
	if name == "" {
		name, _ = os.Hostname()
	}

	a := &agent.Agent{APIBase: *apiBase, StateDir: *stateDir, Keys: keys, HWID: id, Version: version}
	ctx := context.Background()

	if _, err := a.Token(ctx); errors.Is(err, agent.ErrNotEnrolled) {
		if *code == "" {
			log.Fatal("this PC is not enrolled yet: run again with -code AVTO-XXXX-XXXX")
		}
		if err := a.Enroll(ctx, *code, name); err != nil {
			log.Fatalf("enrollment failed: %v", err)
		}
		log.Printf("enrolled as %q", name)
		if _, err := a.Token(ctx); err != nil {
			log.Fatalf("first token failed: %v", err)
		}
	} else if err != nil {
		log.Fatalf("station token: %v", err)
	}

	handler := proxy.New(*frontend, *apiBase, a.Token)
	url := fmt.Sprintf("http://%s/uz/station", *addr)

	if !*noKiosk {
		if _, err := kiosk.Launch(url); err != nil {
			log.Printf("kiosk launch: %v (open %s manually)", err, url)
		}
	}
	log.Printf("avtotest-station %s serving %s", version, url)
	log.Fatal(http.ListenAndServe(*addr, handler))
}

// defaultStateDir keeps the key beside the program data, not in a user
// profile, because the agent runs as a machine service.
func defaultStateDir() string {
	if dir := os.Getenv("ProgramData"); dir != "" {
		return filepath.Join(dir, "AvtoTest", "station")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return ".avtotest-station"
	}
	return filepath.Join(dir, "avtotest-station")
}
