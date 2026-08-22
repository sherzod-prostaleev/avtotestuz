package agent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"avtotest.uz/station/internal/agent"
)

// TestReadLogTailKeepsTheEnd. The last lines before a failure are the entire
// reason the log travels at all; a head-first read would ship a PC's startup
// banner from three months ago and nothing about today.
func TestReadLogTailKeepsTheEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "station.log")

	var b strings.Builder
	for i := 0; i < 5000; i++ {
		b.WriteString("2026/08/22 10:00:00 filler line that pads the log out\n")
	}
	b.WriteString("2026/08/22 12:00:00 enrollment refused: conflict\n")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	tail := agent.ReadLogTail(path)
	if !strings.HasSuffix(tail, "enrollment refused: conflict\n") {
		t.Fatal("the tail does not end with the last line of the log")
	}
	if len(tail) > 28<<10 {
		t.Fatalf("tail is %d bytes, want it bounded", len(tail))
	}
	// A truncated read must not begin mid-line: a half sentence at the top of
	// a support report is worse than one fewer line.
	if strings.HasPrefix(tail, "filler") || strings.HasPrefix(tail, "0:00") {
		t.Fatalf("tail starts mid-line: %.40q", tail)
	}
	if !strings.HasPrefix(tail, "2026/") {
		t.Fatalf("tail does not start at a line boundary: %.40q", tail)
	}
}

// TestReadLogTailReturnsWholeSmallFiles. Most reports come from a PC that has
// only just started, where the whole log is a few lines.
func TestReadLogTailReturnsWholeSmallFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "station.log")
	const body = "line one\nline two\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := agent.ReadLogTail(path); got != body {
		t.Fatalf("ReadLogTail() = %q, want the whole file", got)
	}
}

// TestReadLogTailToleratesAMissingFile. startLogging gives up silently when it
// cannot create a log — a read-only or full disk — and a report with no tail is
// still worth sending. "There is no log" is itself a useful thing to see.
func TestReadLogTailToleratesAMissingFile(t *testing.T) {
	if got := agent.ReadLogTail(filepath.Join(t.TempDir(), "absent.log")); got != "" {
		t.Fatalf("ReadLogTail() = %q, want empty for a missing file", got)
	}
}
