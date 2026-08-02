package admin

import (
	"runtime"
	"testing"
	"time"
)

func TestCPUUtilizationPct(t *testing.T) {
	prev := cpuTimes{total: 1000, idle: 800}
	cur := cpuTimes{total: 1100, idle: 850} // +100 total, +50 idle → 50% busy
	got := cpuUtilizationPct(prev, cur)
	if got < 49.9 || got > 50.1 {
		t.Fatalf("got %v want 50", got)
	}
	if cpuUtilizationPct(cur, prev) != 0 {
		t.Fatal("negative delta should yield 0")
	}
}

func TestRoundPct(t *testing.T) {
	if roundPct(12.345) != 12.35 {
		t.Fatalf("got %v", roundPct(12.345))
	}
	if roundPct(-1) != 0 || roundPct(101) != 100 {
		t.Fatal("clamp failed")
	}
}

func TestHostMetricsSnapshotLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires /proc")
	}
	// Reset cache so this test always exercises a real sample window.
	hostMetricsState.mu.Lock()
	hostMetricsState.snapshot = nil
	hostMetricsState.cpuPrevOK = false
	hostMetricsState.mu.Unlock()

	start := time.Now()
	snap := hostMetricsSnapshot()
	if time.Since(start) < hostCPUSampleWindow {
		t.Fatalf("first sample should wait for CPU window, elapsed=%s", time.Since(start))
	}
	if snap["available"] != true || snap["status"] != "ok" {
		t.Fatalf("snap=%+v", snap)
	}
	for _, key := range []string{"cpu_pct", "ram_pct", "disk_pct"} {
		v, ok := snap[key].(float64)
		if !ok || v < 0 || v > 100 {
			t.Fatalf("%s=%v snap=%+v", key, snap[key], snap)
		}
	}

	// Cached path should be fast and equal within TTL.
	start = time.Now()
	snap2 := hostMetricsSnapshot()
	if time.Since(start) > 50*time.Millisecond {
		t.Fatalf("cached snapshot too slow: %s", time.Since(start))
	}
	if snap2["cpu_pct"] != snap["cpu_pct"] || snap2["ram_pct"] != snap["ram_pct"] {
		t.Fatalf("cache mismatch: %+v vs %+v", snap, snap2)
	}
}

func TestReadCPUTimesAndRAMDisk(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires /proc")
	}
	ct, err := readCPUTimes()
	if err != nil || ct.total == 0 {
		t.Fatalf("cpu=%+v err=%v", ct, err)
	}
	ram, err := readRAMUsedPct()
	if err != nil || ram < 0 || ram > 100 {
		t.Fatalf("ram=%v err=%v", ram, err)
	}
	disk, err := readDiskUsedPct("/")
	if err != nil || disk < 0 || disk > 100 {
		t.Fatalf("disk=%v err=%v", disk, err)
	}
}
