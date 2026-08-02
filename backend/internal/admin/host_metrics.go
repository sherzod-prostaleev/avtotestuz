package admin

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// CPU sample window: short enough for responsive admin UI, long enough that
// /proc/stat jiffies yield a stable utilization percentage.
const hostCPUSampleWindow = 250 * time.Millisecond

// Reuse a fresh snapshot for concurrent admin refreshes within this TTL.
const hostMetricsCacheTTL = time.Second

type cpuTimes struct {
	total uint64
	idle  uint64 // idle + iowait (same basis as top/htop)
}

type hostMetricsCache struct {
	mu        sync.Mutex
	cpuPrev   cpuTimes
	cpuPrevAt time.Time
	cpuPrevOK bool
	at        time.Time
	snapshot  map[string]any
}

var hostMetricsState hostMetricsCache

func hostMetricsSnapshot() map[string]any {
	if runtime.GOOS != "linux" {
		return hostMetricsUnavailable("host metrics require Linux (/proc)")
	}

	hostMetricsState.mu.Lock()
	if hostMetricsState.snapshot != nil && time.Since(hostMetricsState.at) < hostMetricsCacheTTL {
		out := cloneHostMetrics(hostMetricsState.snapshot)
		hostMetricsState.mu.Unlock()
		return out
	}
	prev := hostMetricsState.cpuPrev
	prevOK := hostMetricsState.cpuPrevOK
	prevAt := hostMetricsState.cpuPrevAt
	hostMetricsState.mu.Unlock()

	snap, nextPrev, err := collectHostMetrics(prev, prevOK, prevAt)
	if err != nil {
		return hostMetricsUnavailable(err.Error())
	}

	hostMetricsState.mu.Lock()
	defer hostMetricsState.mu.Unlock()
	// Another goroutine may have filled a fresher cache while we sampled.
	if hostMetricsState.snapshot != nil && time.Since(hostMetricsState.at) < hostMetricsCacheTTL {
		return cloneHostMetrics(hostMetricsState.snapshot)
	}
	hostMetricsState.cpuPrev = nextPrev
	hostMetricsState.cpuPrevAt = time.Now()
	hostMetricsState.cpuPrevOK = true
	hostMetricsState.snapshot = snap
	hostMetricsState.at = time.Now()
	return cloneHostMetrics(snap)
}

func collectHostMetrics(prev cpuTimes, prevOK bool, prevAt time.Time) (map[string]any, cpuTimes, error) {
	cur, err := readCPUTimes()
	if err != nil {
		return nil, cpuTimes{}, err
	}

	var (
		cpuPct   float64
		nextPrev cpuTimes
	)
	if prevOK && time.Since(prevAt) >= hostCPUSampleWindow {
		cpuPct = cpuUtilizationPct(prev, cur)
		nextPrev = cur
	} else {
		time.Sleep(hostCPUSampleWindow)
		cur2, err := readCPUTimes()
		if err != nil {
			return nil, cpuTimes{}, err
		}
		cpuPct = cpuUtilizationPct(cur, cur2)
		nextPrev = cur2
	}

	ramPct, err := readRAMUsedPct()
	if err != nil {
		return nil, cpuTimes{}, err
	}
	diskPct, err := readDiskUsedPct("/")
	if err != nil {
		return nil, cpuTimes{}, err
	}

	return map[string]any{
		"available": true,
		"status":    "ok",
		"cpu_pct":   roundPct(cpuPct),
		"ram_pct":   roundPct(ramPct),
		"disk_pct":  roundPct(diskPct),
		"note":      "sampled from /proc and root filesystem (process host view)",
	}, nextPrev, nil
}

func hostMetricsUnavailable(note string) map[string]any {
	return map[string]any{
		"available": false,
		"status":    "unavailable",
		"cpu_pct":   nil,
		"ram_pct":   nil,
		"disk_pct":  nil,
		"note":      note,
	}
}

func cloneHostMetrics(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func roundPct(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return math.Round(v*100) / 100
}

func cpuUtilizationPct(prev, cur cpuTimes) float64 {
	totalDelta := float64(cur.total - prev.total)
	if totalDelta <= 0 {
		return 0
	}
	idleDelta := float64(cur.idle - prev.idle)
	busy := totalDelta - idleDelta
	if busy < 0 {
		busy = 0
	}
	return (busy / totalDelta) * 100
}

func readCPUTimes() (cpuTimes, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuTimes{}, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return cpuTimes{}, err
		}
		return cpuTimes{}, fmt.Errorf("/proc/stat: empty")
	}
	fields := strings.Fields(sc.Text())
	// cpu user nice system idle iowait irq softirq steal guest guest_nice
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuTimes{}, fmt.Errorf("/proc/stat: unexpected first line")
	}

	vals := make([]uint64, 0, len(fields)-1)
	for _, raw := range fields[1:] {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return cpuTimes{}, fmt.Errorf("/proc/stat: %w", err)
		}
		vals = append(vals, n)
	}

	var total uint64
	for _, n := range vals {
		total += n
	}
	idle := vals[3] // idle
	if len(vals) > 4 {
		idle += vals[4] // iowait
	}
	return cpuTimes{total: total, idle: idle}, nil
}

func readRAMUsedPct() (float64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var memTotal, memAvailable uint64
	var haveTotal, haveAvail bool
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(val))
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "MemTotal":
			memTotal = n
			haveTotal = true
		case "MemAvailable":
			memAvailable = n
			haveAvail = true
		}
		if haveTotal && haveAvail {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	if !haveTotal || memTotal == 0 {
		return 0, fmt.Errorf("/proc/meminfo: MemTotal missing")
	}
	if !haveAvail {
		return 0, fmt.Errorf("/proc/meminfo: MemAvailable missing")
	}
	used := float64(memTotal - memAvailable)
	return (used / float64(memTotal)) * 100, nil
}

func readDiskUsedPct(path string) (float64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	// Match df(1): capacity = used / (used + available), where used excludes
	// reserved blocks that non-root cannot use (bavail vs bfree).
	total := st.Blocks
	free := st.Bfree
	avail := st.Bavail
	if total == 0 {
		return 0, fmt.Errorf("statfs %s: zero blocks", path)
	}
	used := total - free
	denom := used + avail
	if denom == 0 {
		return 0, nil
	}
	return (float64(used) / float64(denom)) * 100, nil
}
