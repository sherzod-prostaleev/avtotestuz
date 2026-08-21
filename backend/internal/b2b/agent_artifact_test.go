package b2b

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeAgent(t *testing.T, body, version string) (binaryPath, versionPath string) {
	t.Helper()
	dir := t.TempDir()
	binaryPath = filepath.Join(dir, "avtotest-station.exe")
	versionPath = filepath.Join(dir, "agent-version")
	if err := os.WriteFile(binaryPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(versionPath, []byte(version+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return binaryPath, versionPath
}

// TestDescribeAgentMatchesTheServedBytes is the property the whole self-update
// path depends on: an agent refuses to install anything whose digest differs
// from the manifest, so a manifest that does not describe the file next to it
// would stop the entire fleet from ever updating.
func TestDescribeAgentMatchesTheServedBytes(t *testing.T) {
	const body = "MZ...pretend this is a 6MB Windows binary..."
	binaryPath, versionPath := writeAgent(t, body, "1.1.0")

	art, err := describeAgent(binaryPath, versionPath)
	if err != nil {
		t.Fatalf("describeAgent() = %v", err)
	}
	sum := sha256.Sum256([]byte(body))
	if art.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha256 = %s, want the digest of the file itself", art.SHA256)
	}
	if art.Size != int64(len(body)) {
		t.Fatalf("size = %d, want %d", art.Size, len(body))
	}
	if art.Version != "1.1.0" {
		t.Fatalf("version = %q, want the trimmed contents of agent-version", art.Version)
	}
}

// TestAgentManifestRefusesWhenNoBinaryIsPresent covers a local build and any
// image where the station stage did not run: answering 200 with an empty
// version would make every classroom PC treat "" as a new release.
func TestAgentManifestRefusesWhenNoBinaryIsPresent(t *testing.T) {
	agentCache = artifactCache{}
	h := &Handler{StationBinaryPath: "/nonexistent/agent.exe", StationVersionPath: "/nonexistent/v"}

	w := httptest.NewRecorder()
	h.agentManifest(w, httptest.NewRequest(http.MethodGet, "/b2b/stations/agent-manifest", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

// TestAgentDownloadIsCacheableOnlyWhenPinnedToAVersion. The manifest is never
// cached and names an exact version; the download is then asked for by that
// name, which is what makes it safe to keep at the edge for a day. Without the
// pin an edge could hand a school yesterday's binary while the manifest
// promises today's digest, and every update would fail its integrity check.
func TestAgentDownloadIsCacheableOnlyWhenPinnedToAVersion(t *testing.T) {
	const body = "AGENT-BYTES"
	binaryPath, versionPath := writeAgent(t, body, "1.1.0")
	agentCache = artifactCache{}
	h := &Handler{StationBinaryPath: binaryPath, StationVersionPath: versionPath}

	pinned := httptest.NewRecorder()
	h.agentDownload(pinned, httptest.NewRequest(http.MethodGet, "/b2b/stations/agent?v=1.1.0", nil))
	if got := pinned.Header().Get("Cache-Control"); got != "public, max-age=86400, immutable" {
		t.Fatalf("pinned Cache-Control = %q, want it cacheable", got)
	}
	served, err := io.ReadAll(pinned.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(served) != body {
		t.Fatalf("served %q, want the binary itself", served)
	}
	sum := sha256.Sum256([]byte(body))
	if got, want := pinned.Header().Get("ETag"), `"`+hex.EncodeToString(sum[:])+`"`; got != want {
		t.Fatalf("ETag = %s, want %s", got, want)
	}

	bare := httptest.NewRecorder()
	h.agentDownload(bare, httptest.NewRequest(http.MethodGet, "/b2b/stations/agent", nil))
	if got := bare.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("unpinned Cache-Control = %q, want no-cache", got)
	}
}

// TestAgentManifestIsNeverCached. This response is how a school learns a fix
// exists; an edge holding it delays every classroom by exactly that long.
func TestAgentManifestIsNeverCached(t *testing.T) {
	binaryPath, versionPath := writeAgent(t, "AGENT", "1.2.3")
	agentCache = artifactCache{}
	h := &Handler{StationBinaryPath: binaryPath, StationVersionPath: versionPath}

	w := httptest.NewRecorder()
	h.agentManifest(w, httptest.NewRequest(http.MethodGet, "/b2b/stations/agent-manifest", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	var env struct {
		Data agentArtifact `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("manifest is not the standard envelope: %v (%s)", err, w.Body.String())
	}
	if env.Data.Version != "1.2.3" || env.Data.SHA256 == "" || env.Data.Size == 0 {
		t.Fatalf("manifest = %+v, want every field populated", env.Data)
	}
}
