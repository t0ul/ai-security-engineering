// Command main is the runnable demo for lesson 01.4, the watched folder.
//
// Run it with:
//
//	go run .
//
// The agent watches an inbox folder for new .txt files. That folder is an
// untrusted boundary: anyone (or any process) can drop a file that is too big,
// still being written, or one of thousands at once. This demo builds a naive
// scanner with none of the ingestion limits, shows each of those three inputs
// biting it, then runs the same inputs through the product's real limits: a size
// cap, a settle delay, and a per-cycle cap.
//
// Nothing here is a real denial of service. The oversized file is a few hundred
// kilobytes, just past the cap, not the gigabytes an actual attack would use. The
// point is that the capped path refuses it after one os.Stat, never reading it, so
// the file's real size never matters.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/t0ul/emaildrop/pkg/agent/pipeline"
	"github.com/t0ul/emaildrop/pkg/agent/watcher"
)

// limits reads the product's real ingestion limits: MaxBytes from the pipeline,
// and the settle delay and per-cycle cap from the watcher's default config. The
// lesson never hardcodes these, so if the daemon changes them, this demo changes
// with it.
func limits() (maxBytes int64, settle time.Duration, perCycle int) {
	cfg := watcher.NewConfig(os.TempDir())
	return pipeline.MaxBytes, cfg.SettleSeconds, cfg.MaxPerCycle
}

// scanned is the outcome of one file in one scan pass.
type scanned struct {
	name      string
	outcome   string
	bytesRead int64 // how much the scanner pulled into memory for this file
}

func main() {
	maxBytes, settle, perCycle := limits()
	fmt.Printf("product limits: size cap %d bytes, settle delay %s, per-cycle cap %d\n",
		maxBytes, settle, perCycle)

	okA := sizeAndSettle(maxBytes, settle)
	okB := perCycleCap(perCycle)

	fmt.Println()
	if okA && okB {
		fmt.Println("attack blocked (the receipt)")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- scenario A: an oversized file and a half-written file ----

func sizeAndSettle(maxBytes int64, settle time.Duration) bool {
	dir := mustTempDir()
	defer os.RemoveAll(dir)

	// A normal email: small, and finished writing a while ago so it has settled.
	writeFile(dir, "clean.txt", "PTA Meeting: Tuesday, September 29th 5:30PM", ago(2*settle))
	// An oversized file, just past the cap, settled. Harmless in size here; the
	// point is that it crosses the limit.
	writeFile(dir, "huge.txt", strings.Repeat("x", int(maxBytes)+1024), ago(2*settle))
	// A half-written file: its modification time is now, as if a process is still
	// writing it this instant.
	writeFile(dir, "partial.txt", "Open Hou", time.Now())

	fmt.Println("\n== before: a naive scan (no limits) ==")
	naive := naiveScan(dir)
	landed := 0
	for _, s := range naive {
		fmt.Printf("  %-12s %-28s read %d bytes\n", s.name, s.outcome, s.bytesRead)
	}
	// The attack lands if the naive scan read the oversized file whole and read the
	// half-written file at all.
	if read(naive, "huge.txt") > maxBytes && read(naive, "partial.txt") > 0 {
		landed = 1
		fmt.Println("  the naive scan read a too-big file whole and read a half-written one")
	}

	fmt.Println("\n== after: the product's size cap + settle delay ==")
	safe := safeScan(dir, maxBytes, settle, 1000)
	for _, s := range safe {
		fmt.Printf("  %-12s %-28s read %d bytes\n", s.name, s.outcome, s.bytesRead)
	}
	blocked := outcome(safe, "huge.txt") == "rejected: oversized" &&
		outcome(safe, "partial.txt") == "skipped: not settled" &&
		outcome(safe, "clean.txt") == "processed"
	if blocked {
		fmt.Println("  oversized refused unread, half-written skipped until it settles, clean one processed")
	}
	return landed == 1 && blocked
}

// ---- scenario B: a flood of files in one scan ----

func perCycleCap(perCycle int) bool {
	dir := mustTempDir()
	defer os.RemoveAll(dir)

	flood := perCycle + 5
	for i := 0; i < flood; i++ {
		writeFile(dir, fmt.Sprintf("mail-%02d.txt", i), "Book fair: October 2", ago(5*time.Second))
	}

	fmt.Printf("\n== before: a naive scan of %d files at once ==\n", flood)
	naive := naiveScan(dir)
	fmt.Printf("  processed %d files in one pass (unbounded)\n", len(naive))

	fmt.Printf("\n== after: the product's per-cycle cap of %d ==\n", perCycle)
	safe := safeScan(dir, 1<<30, 0, perCycle)
	processed, deferred := 0, 0
	for _, s := range safe {
		if s.outcome == "processed" {
			processed++
		} else {
			deferred++
		}
	}
	fmt.Printf("  processed %d this pass, deferred %d to the next (work per cycle is bounded)\n", processed, deferred)
	return len(naive) == flood && processed == perCycle && deferred == flood-perCycle
}

// ---- the two scanners ----

// naiveScan is the watcher I would have written first: list the inbox and read
// every .txt right now, fully, with no size check, no settle delay, and no cap.
// Each of those omissions is one of the three breaks this lesson fixes.
func naiveScan(dir string) []scanned {
	var out []scanned
	for _, name := range txtFiles(dir) {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			out = append(out, scanned{name, "error: " + err.Error(), 0})
			continue
		}
		out = append(out, scanned{name, "processed", int64(len(data))})
	}
	return out
}

// safeScan mirrors the product's watcher and pipeline: skip a file that has not
// been idle for the settle delay (it may still be being written), reject a file
// over the size cap without reading it, and process at most perCycle files in one
// pass. These are the limits imported from the product, not reinvented here.
func safeScan(dir string, maxBytes int64, settle time.Duration, perCycle int) []scanned {
	now := time.Now()
	var ready []string
	for _, name := range txtFiles(dir) {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if now.Sub(fi.ModTime()) < settle {
			continue // not settled: may be half-written
		}
		ready = append(ready, name)
	}
	sort.Strings(ready)

	var out []scanned
	// Report the unsettled files too, so the skip is visible.
	for _, name := range txtFiles(dir) {
		if !contains(ready, name) {
			out = append(out, scanned{name, "skipped: not settled", 0})
		}
	}
	for i, name := range ready {
		if i >= perCycle {
			out = append(out, scanned{name, "deferred: over per-cycle cap", 0})
			continue
		}
		fi, _ := os.Stat(filepath.Join(dir, name))
		if fi.Size() > maxBytes {
			out = append(out, scanned{name, "rejected: oversized", 0}) // never read
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			out = append(out, scanned{name, "error: " + err.Error(), 0})
			continue
		}
		out = append(out, scanned{name, "processed", int64(len(data))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// ---- small helpers ----

func txtFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".txt") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func writeFile(dir, name, body string, mod time.Time) {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		panic(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		panic(err)
	}
}

func ago(d time.Duration) time.Time { return time.Now().Add(-d) }

func mustTempDir() string {
	d, err := os.MkdirTemp("", "lesson-01.4-*")
	if err != nil {
		panic(err)
	}
	return d
}

func read(ss []scanned, name string) int64 {
	for _, s := range ss {
		if s.name == name {
			return s.bytesRead
		}
	}
	return -1
}

func outcome(ss []scanned, name string) string {
	for _, s := range ss {
		if s.name == name {
			return s.outcome
		}
	}
	return ""
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
