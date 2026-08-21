package b2b

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"avtotest.uz/backend/internal/httpx"
)

// The classroom agent used to reach a school exactly once: an admin downloaded
// it from the panel and someone walked it round the room. Every fix after that
// stayed on the server. There was no way to answer "which build is that school
// running?", and no way to push a fix without asking a driving school to
// repeat the whole installation.
//
// These two routes close that. The image already carries the Windows binary it
// serves from the admin panel; the manifest describes it and the download hands
// it over. Both are unauthenticated on purpose: the file here is the plain,
// school-agnostic agent. The secret in a downloaded installer is the config
// trailer the admin panel appends -- the school's installer key -- and that is
// never part of this response. An agent updating itself re-attaches the trailer
// it already holds locally (see the station module's internal/updater), so the
// key never crosses the network again after the first install.
//
// Integrity comes from the SHA-256 in the manifest, which the agent checks
// before it swaps anything into place. That is not a substitute for
// Authenticode signing -- someone who can serve these bytes can serve a
// different manifest too -- but it does close the accidental half of the
// problem: a truncated download, a stale edge cache, a proxy that rewrote the
// body. Signing the release with a key held offline is the next step and is
// tracked in backend/station/security/GOVERNANCE.md.

// agentArtifact is the description of the binary on disk. It is computed once:
// the file is baked into the image and cannot change while the process runs.
type agentArtifact struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

type artifactCache struct {
	once sync.Once
	art  agentArtifact
	err  error
}

var agentCache artifactCache

func (h *Handler) artifact() (agentArtifact, error) {
	agentCache.once.Do(func() {
		agentCache.art, agentCache.err = describeAgent(h.StationBinaryPath, h.StationVersionPath)
	})
	return agentCache.art, agentCache.err
}

func describeAgent(binaryPath, versionPath string) (agentArtifact, error) {
	f, err := os.Open(binaryPath)
	if err != nil {
		return agentArtifact{}, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return agentArtifact{}, err
	}
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return agentArtifact{}, err
	}

	// The version is written beside the binary by the same Docker stage that
	// stamped it into the binary, so the two can never disagree. Reading it
	// out of the PE version resource instead would be one more parser to
	// maintain for no extra guarantee.
	version := ""
	if b, rErr := os.ReadFile(versionPath); rErr == nil {
		version = strings.TrimSpace(string(b))
	}
	return agentArtifact{
		Version: version,
		SHA256:  hex.EncodeToString(sum.Sum(nil)),
		Size:    info.Size(),
	}, nil
}

func (h *Handler) agentManifest(w http.ResponseWriter, r *http.Request) {
	art, err := h.artifact()
	if err != nil || art.Version == "" {
		httpx.Error(w, http.StatusServiceUnavailable, "agent_unavailable",
			"no station agent is present in this build")
		return
	}
	// Never cached: this is the signal that tells a classroom PC a new build
	// exists, and an edge holding it for even an hour delays every school's
	// fix by that hour. It is a few hundred bytes.
	w.Header().Set("Cache-Control", "no-store")
	httpx.Data(w, http.StatusOK, art)
}

func (h *Handler) agentDownload(w http.ResponseWriter, r *http.Request) {
	art, err := h.artifact()
	if err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "agent_unavailable",
			"no station agent is present in this build")
		return
	}
	f, err := os.Open(h.StationBinaryPath)
	if err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "agent_unavailable",
			"no station agent is present in this build")
		return
	}
	defer func() { _ = f.Close() }()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("ETag", `"`+art.SHA256+`"`)
	// Safe to cache hard because the agent asks for a specific version
	// (?v=1.1.0, taken from the manifest it just read) and the manifest is
	// never cached. A school with thirty PCs then pulls ~6 MB from the origin
	// once instead of thirty times.
	if r.URL.Query().Get("v") != "" {
		w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	// ServeContent handles Range and If-None-Match, so a download interrupted
	// by a classroom's flaky connection can resume instead of starting over.
	http.ServeContent(w, r, "avtotest-station.exe", time.Time{}, f)
}
