// Package ir replays an incident from the audit trail. Because every hop in a
// request emits a span under one correlation trace_id to the hash-chained
// gledger log, an incident is fully reconstructable: filter the log by trace_id
// and you get the ordered timeline of exactly what the system did — scrub,
// plan, policy gate, detonation, sanitizer, approval, kill switch. This is the
// M12/M18 forensic payoff: replay the attack end to end, on tamper-evident
// evidence.
package ir

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Event is one decoded audit record.
type Event struct {
	TS      string         `json:"ts"`
	TraceID string         `json:"trace_id"`
	Service string         `json:"service"`
	Span    string         `json:"span"`
	Event   string         `json:"event"`
	Fields  map[string]any `json:"fields"`
}

// Load reads all records from a gledger JSONL audit log.
func Load(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e Event
		if json.Unmarshal([]byte(line), &e) == nil && e.TraceID != "" {
			out = append(out, e)
		}
	}
	return out, sc.Err()
}

// Timeline returns the events for one trace_id in chronological order.
func Timeline(events []Event, traceID string) []Event {
	var out []Event
	for _, e := range events {
		if e.TraceID == traceID {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TS < out[j].TS })
	return out
}

// Traces returns the distinct trace_ids present, newest first by first-seen ts.
func Traces(events []Event) []string {
	seen := map[string]string{} // trace -> earliest ts
	for _, e := range events {
		if cur, ok := seen[e.TraceID]; !ok || e.TS < cur {
			seen[e.TraceID] = e.TS
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return seen[ids[i]] > seen[ids[j]] })
	return ids
}

// Format renders a timeline as readable lines.
func Format(events []Event) string {
	var b strings.Builder
	for _, e := range events {
		fmt.Fprintf(&b, "%s  %-12s %-18s %s\n", e.TS, e.Service, e.Span+"/"+e.Event, fieldsStr(e.Fields))
	}
	return b.String()
}

func fieldsStr(f map[string]any) string {
	if len(f) == 0 {
		return ""
	}
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, f[k]))
	}
	return strings.Join(parts, " ")
}
